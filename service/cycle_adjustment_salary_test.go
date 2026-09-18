package service

import (
	"errors"
	"testing"
	"time"

	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

type adjustmentSalaryRepo struct {
	repository.PayrollRunRepository
	t                         *testing.T
	cycle                     models.PayrollCycle
	salary                    models.SalariesStructure
	salaryErr                 error
	lookupErr                 error
	inTx, locked, closeOnLock bool
	salaryReads, optionReads  int
	optionScope               repository.AccessScope
}

func (r *adjustmentSalaryRepo) FindCycleScoped(scope repository.AccessScope, id uint) (models.PayrollCycle, error) {
	if !scope.CanAccessCompany(r.cycle.CompaniesID) || id != r.cycle.ID {
		return models.PayrollCycle{}, repository.ErrPayrollCycleMissing
	}
	return r.cycle, r.lookupErr
}

func (r *adjustmentSalaryRepo) FindOpenCycleScoped(scope repository.AccessScope, id uint) (models.PayrollCycle, error) {
	cycle, err := r.FindCycleScoped(scope, id)
	if err == nil && cycle.IsClosed {
		err = repository.ErrPayrollCycleClosed
	}
	return cycle, err
}

func (r *adjustmentSalaryRepo) LockCycleForUpdate(uint) (models.PayrollCycle, error) {
	if !r.inTx {
		r.t.Fatal("cycle lock outside transaction")
	}
	r.locked = true
	cycle := r.cycle
	cycle.IsClosed = cycle.IsClosed || r.closeOnLock
	return cycle, nil
}

func (r *adjustmentSalaryRepo) FindEarliestSalaryOnOrBefore(employeeID uint, end time.Time) (models.SalariesStructure, error) {
	if !r.inTx || !r.locked {
		r.t.Fatal("salary validation must follow the cycle lock in the write transaction")
	}
	r.salaryReads++
	if r.salaryErr != nil {
		return models.SalariesStructure{}, r.salaryErr
	}
	salary := r.salary
	if salary.EmployeesID != employeeID || !salary.IsActive || salary.DeletedAt != 0 || salary.EffectiveDate.After(end) {
		return models.SalariesStructure{}, repository.ErrPayrollSalaryMissing
	}
	return salary, nil
}

func (r *adjustmentSalaryRepo) ListCycleEmployeeOptions(scope repository.AccessScope, cycle models.PayrollCycle) ([]repository.CycleEmployeeOption, error) {
	r.optionReads++
	r.optionScope = scope
	if cycle.ID != r.cycle.ID {
		r.t.Fatal("wrong cycle for options")
	}
	return []repository.CycleEmployeeOption{{ID: 9, CompaniesID: cycle.CompaniesID}}, nil
}

type adjustmentEmployees struct {
	repository.EmployeesRepository
	companyID uint
}

func (r adjustmentEmployees) FindBriefScoped(repository.AccessScope, uint) (models.Employees, error) {
	return models.Employees{ID: 9, CompaniesID: r.companyID, IsActive: true}, nil
}

type adjustmentEarningTypes struct {
	repository.EarningTypeRepository
}

func (adjustmentEarningTypes) Find(uint) (models.EarningType, error) {
	return models.EarningType{ID: 3, CompaniesID: 7}, nil
}

type adjustmentDeductionTypes struct {
	repository.DeductionTypeRepository
}

func (adjustmentDeductionTypes) Find(uint) (models.DeductionType, error) {
	return models.DeductionType{ID: 3, CompaniesID: 7}, nil
}

func adjustmentDate(day int) time.Time {
	return time.Date(2026, 9, day, 0, 0, 0, 0, timeutil.BangkokLocation)
}

func adjustmentOperation(kind, operation string, r *adjustmentSalaryRepo, employeeCompany uint) (func() error, *int) {
	scope := repository.AccessScope{User: models.Employees{ID: 1}, AuthorizedCompanyIDs: []uint{7}, DataScope: models.ScopeCompanies}
	employees := adjustmentEmployees{companyID: employeeCompany}
	if kind == "earning" {
		repo := &adjustmentEarningRepo{cycles: r}
		svc := NewCycleEarningService(repo, r, employees, adjustmentEarningTypes{})
		return func() error {
			input := models.PayrollCycleEarning{CycleID: 4, CompaniesID: 7, EmployeesID: 9, EarningTypeID: 3, Amount: 100}
			if operation == "delete" {
				return svc.Delete(scope, 1)
			}
			if operation == "update" {
				_, err := svc.Update(scope, 1, input)
				return err
			}
			_, err := svc.Create(scope, input)
			return err
		}, &repo.writes
	}
	repo := &adjustmentDeductionRepo{cycles: r}
	svc := NewCycleDeductionService(repo, r, employees, adjustmentDeductionTypes{})
	return func() error {
		input := models.PayrollCycleDeduction{CycleID: 4, CompaniesID: 7, EmployeesID: 9, DeductionTypeID: 3, Amount: 100}
		if operation == "delete" {
			return svc.Delete(scope, 1)
		}
		if operation == "update" {
			_, err := svc.Update(scope, 1, input)
			return err
		}
		_, err := svc.Create(scope, input)
		return err
	}, &repo.writes
}

type adjustmentEarningRepo struct {
	repository.CycleEarningRepository
	cycles *adjustmentSalaryRepo
	writes int
}

func (r *adjustmentEarningRepo) WithinTx(fn func(repository.CycleEarningRepository, repository.PayrollRunRepository) error) error {
	r.cycles.inTx = true
	defer func() { r.cycles.inTx = false }()
	return fn(r, r.cycles)
}
func (r *adjustmentEarningRepo) FindScoped(repository.AccessScope, uint) (models.PayrollCycleEarning, error) {
	return models.PayrollCycleEarning{ID: 1, CycleID: 4, CompaniesID: 7, EmployeesID: 9, EarningTypeID: 3}, nil
}
func (r *adjustmentEarningRepo) FindDetail(uint) (models.PayrollCycleEarning, error) {
	return models.PayrollCycleEarning{ID: 1}, nil
}
func (r *adjustmentEarningRepo) write(audit *models.AuditLog) error {
	if !r.cycles.inTx || !r.cycles.locked {
		r.cycles.t.Fatal("write outside locked transaction")
	}
	if audit == nil {
		r.cycles.t.Fatal("missing audit")
	}
	r.writes++
	return nil
}
func (r *adjustmentEarningRepo) Create(_ *models.PayrollCycleEarning, audit *models.AuditLog) error {
	return r.write(audit)
}
func (r *adjustmentEarningRepo) UpdateFields(_ *models.PayrollCycleEarning, _ map[string]any, audit *models.AuditLog) error {
	return r.write(audit)
}
func (r *adjustmentEarningRepo) Delete(_ *models.PayrollCycleEarning, audit *models.AuditLog) error {
	return r.write(audit)
}

type adjustmentDeductionRepo struct {
	repository.CycleDeductionRepository
	cycles *adjustmentSalaryRepo
	writes int
}

func (r *adjustmentDeductionRepo) WithinTx(fn func(repository.CycleDeductionRepository, repository.PayrollRunRepository) error) error {
	r.cycles.inTx = true
	defer func() { r.cycles.inTx = false }()
	return fn(r, r.cycles)
}
func (r *adjustmentDeductionRepo) FindScoped(repository.AccessScope, uint) (models.PayrollCycleDeduction, error) {
	return models.PayrollCycleDeduction{ID: 1, CycleID: 4, CompaniesID: 7, EmployeesID: 9, DeductionTypeID: 3}, nil
}
func (r *adjustmentDeductionRepo) FindDetail(uint) (models.PayrollCycleDeduction, error) {
	return models.PayrollCycleDeduction{ID: 1}, nil
}
func (r *adjustmentDeductionRepo) write(audit *models.AuditLog) error {
	if !r.cycles.inTx || !r.cycles.locked {
		r.cycles.t.Fatal("write outside locked transaction")
	}
	if audit == nil {
		r.cycles.t.Fatal("missing audit")
	}
	r.writes++
	return nil
}
func (r *adjustmentDeductionRepo) Create(_ *models.PayrollCycleDeduction, audit *models.AuditLog) error {
	return r.write(audit)
}
func (r *adjustmentDeductionRepo) UpdateFields(_ *models.PayrollCycleDeduction, _ map[string]any, audit *models.AuditLog) error {
	return r.write(audit)
}
func (r *adjustmentDeductionRepo) Delete(_ *models.PayrollCycleDeduction, audit *models.AuditLog) error {
	return r.write(audit)
}

func TestCycleAdjustmentsRequireSalaryForSelectedCycle(t *testing.T) {
	for _, kind := range []string{"earning", "deduction"} {
		for _, operation := range []string{"create", "update"} {
			for _, tc := range []struct {
				name    string
				salary  models.SalariesStructure
				allowed bool
			}{
				{"existing salary", models.SalariesStructure{EmployeesID: 9, IsActive: true, EffectiveDate: adjustmentDate(1)}, true},
				{"mid-cycle salary", models.SalariesStructure{EmployeesID: 9, IsActive: true, EffectiveDate: adjustmentDate(15)}, true},
				{"last-day salary", models.SalariesStructure{EmployeesID: 9, IsActive: true, EffectiveDate: adjustmentDate(30)}, true},
				{"future-only salary", models.SalariesStructure{EmployeesID: 9, IsActive: true, EffectiveDate: adjustmentDate(31)}, false},
				{"disabled salary", models.SalariesStructure{EmployeesID: 9, IsActive: false, EffectiveDate: adjustmentDate(1)}, false},
				{"deleted salary", models.SalariesStructure{EmployeesID: 9, IsActive: true, DeletedAt: 1, EffectiveDate: adjustmentDate(1)}, false},
				{"missing salary", models.SalariesStructure{}, false},
			} {
				t.Run(kind+"/"+operation+"/"+tc.name, func(t *testing.T) {
					r := newAdjustmentSalaryRepo(t)
					r.salary = tc.salary
					call, writes := adjustmentOperation(kind, operation, r, 7)
					err := call()
					if tc.allowed {
						if err != nil || *writes != 1 {
							t.Fatalf("err=%v writes=%d", err, *writes)
						}
					} else if !errors.Is(err, ErrCycleEmployeeNoSalary) || *writes != 0 {
						t.Fatalf("err=%v writes=%d; want salary conflict without writes", err, *writes)
					}
				})
			}
		}
	}
}

func newAdjustmentSalaryRepo(t *testing.T) *adjustmentSalaryRepo {
	return &adjustmentSalaryRepo{t: t, cycle: models.PayrollCycle{ID: 4, CompaniesID: 7, StartDate: adjustmentDate(1), EndDate: timeutil.DayEnd(adjustmentDate(30))}}
}

func TestCycleAdjustmentsKeepExistingSafeguards(t *testing.T) {
	for _, kind := range []string{"earning", "deduction"} {
		for _, operation := range []string{"create", "update"} {
			t.Run(kind+"/"+operation+"/database error", func(t *testing.T) {
				r := newAdjustmentSalaryRepo(t)
				r.salaryErr = errors.New("database unavailable")
				call, writes := adjustmentOperation(kind, operation, r, 7)
				if err := call(); !errors.Is(err, r.salaryErr) || *writes != 0 {
					t.Fatalf("err=%v writes=%d", err, *writes)
				}
			})
			t.Run(kind+"/"+operation+"/company mismatch", func(t *testing.T) {
				r := newAdjustmentSalaryRepo(t)
				call, writes := adjustmentOperation(kind, operation, r, 8)
				if err := call(); err == nil || *writes != 0 || r.salaryReads != 0 {
					t.Fatalf("err=%v writes=%d salaryReads=%d", err, *writes, r.salaryReads)
				}
			})
		}
		for _, operation := range []string{"create", "update", "delete"} {
			t.Run(kind+"/"+operation+"/closed while waiting for lock", func(t *testing.T) {
				r := newAdjustmentSalaryRepo(t)
				r.closeOnLock = true
				call, writes := adjustmentOperation(kind, operation, r, 7)
				if err := call(); err == nil || *writes != 0 || r.salaryReads != 0 {
					t.Fatalf("err=%v writes=%d salaryReads=%d", err, *writes, r.salaryReads)
				}
			})
		}
		t.Run(kind+"/delete after salary disabled", func(t *testing.T) {
			r := newAdjustmentSalaryRepo(t)
			call, writes := adjustmentOperation(kind, "delete", r, 7)
			if err := call(); err != nil || *writes != 1 || r.salaryReads != 0 {
				t.Fatalf("err=%v writes=%d salaryReads=%d", err, *writes, r.salaryReads)
			}
		})
	}
}

func TestCycleEmployeeOptionsScope(t *testing.T) {
	for _, tc := range []struct {
		name      string
		companies []uint
		scope     models.DataScope
		allowed   bool
	}{
		{"accessible company dropdown", []uint{7}, "", true},
		{"explicit own scope", []uint{7}, models.ScopeOwn, true},
		{"different company", []uint{8}, models.ScopeCompanies, false},
		{"empty company access", nil, models.ScopeCompanies, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newAdjustmentSalaryRepo(t)
			scope := repository.AccessScope{User: models.Employees{ID: 9}, AuthorizedCompanyIDs: tc.companies, DataScope: tc.scope}
			options, err := NewPayrollRunService(r).CycleEmployeeOptions(scope, 4)
			if !tc.allowed {
				if err == nil || r.optionReads != 0 {
					t.Fatalf("err=%v optionReads=%d", err, r.optionReads)
				}
				return
			}
			if err != nil || len(options) != 1 {
				t.Fatalf("options=%v err=%v", options, err)
			}
			want := tc.scope
			if want == "" {
				want = models.ScopeCompanies
			}
			if r.optionScope.DataScope != want {
				t.Fatalf("scope=%v want=%v", r.optionScope.DataScope, want)
			}
		})
	}
}

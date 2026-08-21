package service

import (
	"errors"
	"testing"
	"time"

	"payroll-backend/models"
	"payroll-backend/repository"
)

// fakeSalaryRepo is an in-memory SalariesStructureRepository that records the order of the calls a write path makes, and whether each one ran inside a transaction.
type fakeSalaryRepo struct {
	records      []models.SalariesStructure
	usage        map[uint]int64
	closedCycles int64
	findRecord   models.SalariesStructure
	activateErr  error

	txDepth   int
	calls     []string
	outsideTx []string
}

// record notes one repository call plus whether a transaction was open when it happened.
func (f *fakeSalaryRepo) record(name string) {
	f.calls = append(f.calls, name)
	if f.txDepth == 0 {
		f.outsideTx = append(f.outsideTx, name)
	}
}

func (f *fakeSalaryRepo) WithinTx(fn func(repository.SalariesStructureRepository) error) error {
	f.txDepth++
	defer func() { f.txDepth-- }()
	return fn(f)
}

func (f *fakeSalaryRepo) LockCompanyCycles(uint) error {
	f.record("LockCompanyCycles")
	return nil
}

func (f *fakeSalaryRepo) ListAllRecordsUnscoped(uint, uint) ([]models.SalariesStructure, error) {
	f.record("ListAllRecordsUnscoped")
	return f.records, nil
}

func (f *fakeSalaryRepo) ClosedCycleUsage([]uint) (map[uint]int64, error) {
	f.record("ClosedCycleUsage")
	if f.usage == nil {
		return map[uint]int64{}, nil
	}
	return f.usage, nil
}

func (f *fakeSalaryRepo) CountClosedCyclesCovering(uint, time.Time) (int64, error) {
	f.record("CountClosedCyclesCovering")
	return f.closedCycles, nil
}

func (f *fakeSalaryRepo) ArchiveRecords([]repository.ArchiveDecision) error {
	f.record("ArchiveRecords")
	return nil
}

func (f *fakeSalaryRepo) ArchiveAndCreate([]repository.ArchiveDecision, *models.SalariesStructure) error {
	f.record("ArchiveAndCreate")
	return nil
}

func (f *fakeSalaryRepo) FindRecord(uint) (models.SalariesStructure, error) {
	f.record("FindRecord")
	return f.findRecord, nil
}

func (f *fakeSalaryRepo) DeleteRecord(*models.SalariesStructure) error {
	f.record("DeleteRecord")
	return nil
}

func (f *fakeSalaryRepo) ActivateLatest(uint) error {
	f.record("ActivateLatest")
	return f.activateErr
}

// Outside the write paths these tests exercise.
func (f *fakeSalaryRepo) ListEmployeesWithSalary(repository.AccessScope) ([]models.Employees, error) {
	return nil, nil
}

func (f *fakeSalaryRepo) ListRecordsForEmployee(uint) ([]models.SalariesStructure, error) {
	return nil, nil
}

// fakeSalaryEmployees answers the one employee lookup the salary service makes.
type fakeSalaryEmployees struct {
	employee models.Employees
}

func (f *fakeSalaryEmployees) FindBriefScoped(repository.AccessScope, uint) (models.Employees, error) {
	return f.employee, nil
}

func (f *fakeSalaryEmployees) ListScoped(repository.AccessScope) ([]models.Employees, error) {
	return nil, nil
}

func (f *fakeSalaryEmployees) FindScoped(repository.AccessScope, uint) (models.Employees, error) {
	return models.Employees{}, nil
}

func (f *fakeSalaryEmployees) FindRole(uint) (models.Roles, error) { return models.Roles{}, nil }
func (f *fakeSalaryEmployees) Create(*models.Employees) error      { return nil }
func (f *fakeSalaryEmployees) Delete(*models.Employees) error      { return nil }

func (f *fakeSalaryEmployees) FindDetail(uint) (models.Employees, error) {
	return models.Employees{}, nil
}

func (f *fakeSalaryEmployees) UpdateFields(*models.Employees, map[string]any) error { return nil }

// salaryScope is a caller authorized for company 1.
func salaryScope() repository.AccessScope {
	return repository.NewAccessScope(models.Employees{ID: 1, CompaniesID: 1}, models.ScopeCompanies, []uint{1})
}

// TestSalarySaveLocksCyclesInsideOneTransaction pins the rule the closed-cycle check depends on: the lock is taken first, and every read and write of the save runs inside the same transaction.
func TestSalarySaveLocksCyclesInsideOneTransaction(t *testing.T) {
	repo := &fakeSalaryRepo{}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	_, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID:   5,
		CompaniesID:   1,
		BaseSalaries:  30000,
		EffectiveDate: "2026-02-01",
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if len(repo.calls) == 0 || repo.calls[0] != "LockCompanyCycles" {
		t.Fatalf("calls = %v, want LockCompanyCycles first", repo.calls)
	}
	if len(repo.outsideTx) != 0 {
		t.Errorf("calls outside the transaction: %v", repo.outsideTx)
	}
	if repo.calls[len(repo.calls)-1] != "ArchiveAndCreate" {
		t.Errorf("calls = %v, want the write last", repo.calls)
	}
}

// TestSalarySaveRejectsDateInClosedCycleWithoutWriting covers the check itself: a date inside a closed cycle must fail before anything is written.
func TestSalarySaveRejectsDateInClosedCycleWithoutWriting(t *testing.T) {
	repo := &fakeSalaryRepo{closedCycles: 1}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	_, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID:   5,
		CompaniesID:   1,
		BaseSalaries:  30000,
		EffectiveDate: "2026-02-01",
	})
	if !errors.Is(err, ErrSalaryDateInClosed) {
		t.Fatalf("Save error = %v, want %v", err, ErrSalaryDateInClosed)
	}
	for _, call := range repo.calls {
		if call == "ArchiveAndCreate" || call == "ArchiveRecords" {
			t.Fatalf("calls = %v, want no write after the closed-cycle check failed", repo.calls)
		}
	}
}

// TestSalaryDeleteErasesAndPromotesInOneTransaction pins the pair that must never come apart: erasing the current record and promoting the previous one.
func TestSalaryDeleteErasesAndPromotesInOneTransaction(t *testing.T) {
	repo := &fakeSalaryRepo{
		findRecord: models.SalariesStructure{ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if err := NewSalariesStructureService(repo, employees).DeleteRecord(salaryScope(), 9); err != nil {
		t.Fatalf("DeleteRecord returned error: %v", err)
	}

	if len(repo.outsideTx) != 1 || repo.outsideTx[0] != "FindRecord" {
		t.Errorf("calls outside the transaction = %v, want only the FindRecord that resolves access", repo.outsideTx)
	}
	want := []string{"FindRecord", "LockCompanyCycles", "ClosedCycleUsage", "DeleteRecord", "ActivateLatest"}
	if len(repo.calls) != len(want) {
		t.Fatalf("calls = %v, want %v", repo.calls, want)
	}
	for i, call := range want {
		if repo.calls[i] != call {
			t.Fatalf("calls = %v, want %v", repo.calls, want)
		}
	}
}

// TestSalaryDeleteReportsPromotionFailure covers the failure the transaction exists for: if the promotion fails the caller must see it, so the rollback that follows is not silent.
func TestSalaryDeleteReportsPromotionFailure(t *testing.T) {
	promoteErr := errors.New("promotion failed")
	repo := &fakeSalaryRepo{
		findRecord:  models.SalariesStructure{ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true},
		activateErr: promoteErr,
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	err := NewSalariesStructureService(repo, employees).DeleteRecord(salaryScope(), 9)
	if !errors.Is(err, promoteErr) {
		t.Fatalf("DeleteRecord error = %v, want %v", err, promoteErr)
	}
}

// TestSalaryDeleteRefusesRecordUsedByClosedCycle keeps the audit rule inside the transaction.
func TestSalaryDeleteRefusesRecordUsedByClosedCycle(t *testing.T) {
	repo := &fakeSalaryRepo{
		findRecord: models.SalariesStructure{ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true},
		usage:      map[uint]int64{9: 1},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	err := NewSalariesStructureService(repo, employees).DeleteRecord(salaryScope(), 9)
	if !errors.Is(err, ErrSalaryRecordInUse) {
		t.Fatalf("DeleteRecord error = %v, want %v", err, ErrSalaryRecordInUse)
	}
	for _, call := range repo.calls {
		if call == "DeleteRecord" {
			t.Fatal("DeleteRecord was called for a record a closed cycle used")
		}
	}
}

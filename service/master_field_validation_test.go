package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"strings"
	"testing"
)

type validationWorkLocationRepo struct {
	repository.WorkLocationRepository
	writes int
	name   string
	audit  *models.AuditLog
}

func (r *validationWorkLocationRepo) FindScoped(repository.AccessScope, uint) (models.WorkLocation, error) {
	return models.WorkLocation{ID: 1, CompaniesID: 1}, nil
}
func (r *validationWorkLocationRepo) Create(row *models.WorkLocation, a *models.AuditLog) error {
	r.writes++
	r.name = row.Name
	r.audit = a
	return nil
}
func (r *validationWorkLocationRepo) Save(row *models.WorkLocation, a *models.AuditLog) error {
	return r.Create(row, a)
}
func TestWorkLocationValidationWrites(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, name := range []string{"", "   ", strings.Repeat("\u0e01", 200), strings.Repeat("\U0001f600", 200), strings.Repeat("\u0e01", 201), "bad\x00name"} {
			r := &validationWorkLocationRepo{}
			svc := NewWorkLocationService(r)
			var err error
			if update {
				_, err = svc.Update(configTestScope(), 1, name)
			} else {
				_, err = svc.Create(configTestScope(), models.WorkLocation{CompaniesID: 1, Name: name})
			}
			valid := name == strings.Repeat("\u0e01", 200) || name == strings.Repeat("\U0001f600", 200)
			if valid {
				if err != nil || r.writes != 1 || r.name != name || r.audit == nil {
					t.Fatalf("valid WorkLocation: %v writes=%d", err, r.writes)
				}
			} else if apperr.KindOf(err) != apperr.KindValidation || r.writes != 0 {
				t.Fatalf("invalid WorkLocation: %v writes=%d", err, r.writes)
			}
		}
	}
}

type validationDepartmentRepo struct {
	repository.DepartmentRepository
	writes int
	name   string
	audit  *models.AuditLog
}

func (r *validationDepartmentRepo) FindScoped(repository.AccessScope, uint) (models.Department, error) {
	return models.Department{ID: 1, CompaniesID: 1}, nil
}
func (r *validationDepartmentRepo) Create(row *models.Department, a *models.AuditLog) error {
	r.writes++
	r.name = row.DeptName
	r.audit = a
	return nil
}
func (r *validationDepartmentRepo) Save(row *models.Department, a *models.AuditLog) error {
	return r.Create(row, a)
}
func TestDepartmentValidationWrites(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, name := range []string{"", "   ", strings.Repeat("\u0e01", 255), strings.Repeat("\U0001f600", 255), strings.Repeat("\u0e01", 256), "bad\x00name"} {
			r := &validationDepartmentRepo{}
			svc := NewDepartmentService(r)
			var err error
			if update {
				_, err = svc.Update(configTestScope(), 1, name, 1)
			} else {
				_, err = svc.Create(configTestScope(), models.Department{CompaniesID: 1, DeptName: name, CategoryID: 1})
			}
			valid := name == strings.Repeat("\u0e01", 255) || name == strings.Repeat("\U0001f600", 255)
			if valid {
				if err != nil || r.writes != 1 || r.name != name || r.audit == nil {
					t.Fatalf("valid Department: %v writes=%d", err, r.writes)
				}
			} else if apperr.KindOf(err) != apperr.KindValidation || r.writes != 0 {
				t.Fatalf("invalid Department: %v writes=%d", err, r.writes)
			}
		}
	}
}

type validationPositionRepo struct {
	repository.PositionRepository
	writes int
	name   string
	audit  *models.AuditLog
}

func (r *validationPositionRepo) FindScoped(repository.AccessScope, uint) (models.Position, error) {
	return models.Position{ID: 1, CompaniesID: 1}, nil
}
func (r *validationPositionRepo) Create(row *models.Position, a *models.AuditLog) error {
	r.writes++
	r.name = row.PosName
	r.audit = a
	return nil
}
func (r *validationPositionRepo) Save(row *models.Position, a *models.AuditLog) error {
	return r.Create(row, a)
}
func TestPositionValidationWrites(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, name := range []string{"", "   ", strings.Repeat("\u0e01", 255), strings.Repeat("\U0001f600", 255), strings.Repeat("\u0e01", 256), "bad\x00name"} {
			r := &validationPositionRepo{}
			svc := NewPositionService(r)
			var err error
			if update {
				_, err = svc.Update(configTestScope(), 1, name)
			} else {
				_, err = svc.Create(configTestScope(), models.Position{CompaniesID: 1, PosName: name})
			}
			valid := name == strings.Repeat("\u0e01", 255) || name == strings.Repeat("\U0001f600", 255)
			if valid {
				if err != nil || r.writes != 1 || r.name != name || r.audit == nil {
					t.Fatalf("valid Position: %v writes=%d", err, r.writes)
				}
			} else if apperr.KindOf(err) != apperr.KindValidation || r.writes != 0 {
				t.Fatalf("invalid Position: %v writes=%d", err, r.writes)
			}
		}
	}
}

type validationEmploymentTypeRepo struct {
	repository.EmploymentTypeRepository
	writes int
	name   string
	audit  *models.AuditLog
}

func (r *validationEmploymentTypeRepo) FindScoped(repository.AccessScope, uint) (models.EmploymentType, error) {
	return models.EmploymentType{ID: 1, CompaniesID: 1}, nil
}
func (r *validationEmploymentTypeRepo) Create(row *models.EmploymentType, a *models.AuditLog) error {
	r.writes++
	r.name = row.Name
	r.audit = a
	return nil
}
func (r *validationEmploymentTypeRepo) Save(row *models.EmploymentType, a *models.AuditLog) error {
	return r.Create(row, a)
}
func TestEmploymentTypeValidationWrites(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, name := range []string{"", "   ", strings.Repeat("\u0e01", 100), strings.Repeat("\U0001f600", 100), strings.Repeat("\u0e01", 101), "bad\x00name"} {
			r := &validationEmploymentTypeRepo{}
			svc := NewEmploymentTypeService(r)
			var err error
			if update {
				_, err = svc.Update(configTestScope(), 1, name, "")
			} else {
				_, err = svc.Create(configTestScope(), models.EmploymentType{CompaniesID: 1, Name: name})
			}
			valid := name == strings.Repeat("\u0e01", 100) || name == strings.Repeat("\U0001f600", 100)
			if valid {
				if err != nil || r.writes != 1 || r.name != name || r.audit == nil {
					t.Fatalf("valid EmploymentType: %v writes=%d", err, r.writes)
				}
			} else if apperr.KindOf(err) != apperr.KindValidation || r.writes != 0 {
				t.Fatalf("invalid EmploymentType: %v writes=%d", err, r.writes)
			}
		}
	}
}

type validationEarningTypeRepo struct {
	repository.EarningTypeRepository
	writes int
	name   string
	audit  *models.AuditLog
}

func (r *validationEarningTypeRepo) FindScoped(repository.AccessScope, uint) (models.EarningType, error) {
	return models.EarningType{ID: 1, CompaniesID: 1}, nil
}
func (r *validationEarningTypeRepo) Create(row *models.EarningType, a *models.AuditLog) error {
	r.writes++
	r.name = row.Name
	r.audit = a
	return nil
}
func (r *validationEarningTypeRepo) Save(row *models.EarningType, a *models.AuditLog) error {
	return r.Create(row, a)
}
func TestEarningTypeValidationWrites(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, name := range []string{"", "   ", strings.Repeat("\u0e01", 200), strings.Repeat("\U0001f600", 200), strings.Repeat("\u0e01", 201), "bad\x00name"} {
			r := &validationEarningTypeRepo{}
			svc := NewEarningTypeService(r)
			var err error
			if update {
				_, err = svc.Update(configTestScope(), 1, models.EarningType{Name: name})
			} else {
				_, err = svc.Create(configTestScope(), models.EarningType{CompaniesID: 1, Name: name})
			}
			valid := name == strings.Repeat("\u0e01", 200) || name == strings.Repeat("\U0001f600", 200)
			if valid {
				if err != nil || r.writes != 1 || r.name != name || r.audit == nil {
					t.Fatalf("valid EarningType: %v writes=%d", err, r.writes)
				}
			} else if apperr.KindOf(err) != apperr.KindValidation || r.writes != 0 {
				t.Fatalf("invalid EarningType: %v writes=%d", err, r.writes)
			}
		}
	}
}

type validationDeductionTypeRepo struct {
	repository.DeductionTypeRepository
	writes int
	name   string
	audit  *models.AuditLog
}

func (r *validationDeductionTypeRepo) FindScoped(repository.AccessScope, uint) (models.DeductionType, error) {
	return models.DeductionType{ID: 1, CompaniesID: 1}, nil
}
func (r *validationDeductionTypeRepo) Create(row *models.DeductionType, a *models.AuditLog) error {
	r.writes++
	r.name = row.Name
	r.audit = a
	return nil
}
func (r *validationDeductionTypeRepo) Save(row *models.DeductionType, a *models.AuditLog) error {
	return r.Create(row, a)
}
func TestDeductionTypeValidationWrites(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, name := range []string{"", "   ", strings.Repeat("\u0e01", 200), strings.Repeat("\U0001f600", 200), strings.Repeat("\u0e01", 201), "bad\x00name"} {
			r := &validationDeductionTypeRepo{}
			svc := NewDeductionTypeService(r)
			var err error
			if update {
				_, err = svc.Update(configTestScope(), 1, models.DeductionType{Name: name})
			} else {
				_, err = svc.Create(configTestScope(), models.DeductionType{CompaniesID: 1, Name: name})
			}
			valid := name == strings.Repeat("\u0e01", 200) || name == strings.Repeat("\U0001f600", 200)
			if valid {
				if err != nil || r.writes != 1 || r.name != name || r.audit == nil {
					t.Fatalf("valid DeductionType: %v writes=%d", err, r.writes)
				}
			} else if apperr.KindOf(err) != apperr.KindValidation || r.writes != 0 {
				t.Fatalf("invalid DeductionType: %v writes=%d", err, r.writes)
			}
		}
	}
}

type validationRoleRepo struct {
	repository.RoleRepository
	writes int
	name   string
	audit  *models.AuditLog
}

func (r *validationRoleRepo) FindScoped(repository.AccessScope, uint) (models.Roles, error) {
	return models.Roles{ID: 1, CompaniesID: 1}, nil
}
func (r *validationRoleRepo) Create(row *models.Roles) error {
	r.writes++
	r.name = row.RolesName
	return nil
}
func (r *validationRoleRepo) UpdateName(row *models.Roles, name string) error {
	row.RolesName = name
	return r.Create(row)
}
func (r *validationRoleRepo) WithinTx(fn func(repository.RoleRepository) error) error { return fn(r) }
func (r *validationRoleRepo) AppendAuditLog(a *models.AuditLog) error                 { r.audit = a; return nil }
func TestRoleValidationWrites(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, name := range []string{"", "   ", strings.Repeat("\u0e01", 255), strings.Repeat("\U0001f600", 255), strings.Repeat("\u0e01", 256), "bad\x00name"} {
			r := &validationRoleRepo{}
			svc := NewRoleService(r)
			var err error
			if update {
				_, err = svc.Update(configTestScope(), 1, name)
			} else {
				_, err = svc.Create(configTestScope(), models.Roles{CompaniesID: 1, RolesName: name})
			}
			valid := name == strings.Repeat("\u0e01", 255) || name == strings.Repeat("\U0001f600", 255)
			if valid {
				if err != nil || r.writes != 1 || r.name != name || r.audit == nil {
					t.Fatalf("valid Role: %v writes=%d", err, r.writes)
				}
			} else if apperr.KindOf(err) != apperr.KindValidation || r.writes != 0 {
				t.Fatalf("invalid Role: %v writes=%d", err, r.writes)
			}
		}
	}
}
func TestCalendarAndCategoryValidationBeforeIO(t *testing.T) {
	for _, name := range []string{strings.Repeat("\u0e01", 201), "bad\x00name"} {
		if _, _, err := parseHolidayFields(name, "2026-10-01"); apperr.KindOf(err) != apperr.KindValidation {
			t.Fatal(err)
		}
		if _, err := validateDayOff(name, 1); apperr.KindOf(err) != apperr.KindValidation {
			t.Fatal(err)
		}
	}
	if _, err := NewDepartmentCategoryService(nil).Create(configTestScope(), strings.Repeat("\u0e01", 101)); apperr.KindOf(err) != apperr.KindValidation {
		t.Fatal(err)
	}
	if _, err := NewPayrollRunService(nil).CreateCycle(configTestScope(), strings.Repeat("\u0e01", 256), "2026-10-01", "2026-10-31", 1); apperr.KindOf(err) != apperr.KindValidation {
		t.Fatal(err)
	}
}

func (r *validationEarningTypeRepo) UpdateFields(row *models.EarningType, fields map[string]any, audit *models.AuditLog) error {
	row.Name = fields["name"].(string)
	return r.Create(row, audit)
}

func (r *validationDeductionTypeRepo) UpdateFields(row *models.DeductionType, fields map[string]any, audit *models.AuditLog) error {
	row.Name = fields["name"].(string)
	return r.Create(row, audit)
}

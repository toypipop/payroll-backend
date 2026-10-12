package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrEmployeeNotFound covers both "no such row" and "out of the caller's scope" so scoping never leaks other tenants' records.
var ErrEmployeeNotFound = apperr.NotFound("Employee not found or access denied")

// ErrRoleNotFound means the referenced role row does not exist.
var ErrRoleNotFound = apperr.NotFound("Role not found")

// ClearField is the value to place in an update map to null a nullable column.
var ClearField = struct{}{}

// EmployeesRepository is the persistence boundary for the employees domain.
type EmployeesRepository interface {
	WithinTx(func(EmployeesRepository) error) error
	ListScoped(scope AccessScope, q PageQuery) (Page[models.Employees], error)
	ListActiveOptions(scope AccessScope) ([]EmployeeOption, error)
	FindScoped(scope AccessScope, id uint) (models.Employees, error)
	FindBriefScoped(scope AccessScope, id uint) (models.Employees, error)
	FindRole(roleID uint) (models.Roles, error)
	ValidateCompanyReferences(employee models.Employees) error
	Create(employee *models.Employees) error
	UpdateFields(employee *models.Employees, fields map[string]any) error
	Delete(employee *models.Employees) error
	FindDetail(id uint) (models.Employees, error)
	AppendAuditLog(*models.AuditLog) error
}

func (r *gormEmployeesRepository) WithinTx(fn func(EmployeesRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error { return fn(&gormEmployeesRepository{db: tx}) })
}

type gormEmployeesRepository struct {
	db *gorm.DB
}

// NewEmployeesRepository returns the GORM-backed implementation on the given connection.
func NewEmployeesRepository(db *gorm.DB) EmployeesRepository {
	return &gormEmployeesRepository{db: db}
}

func (r *gormEmployeesRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee("employees.companies_id", "employees.id"))
}

func (r *gormEmployeesRepository) ListScoped(scope AccessScope, q PageQuery) (Page[models.Employees], error) {
	name := "concat_ws(' ', employees.first_name, employees.last_name)"
	order, err := pageOrder(q, map[string]string{
		"employees_code": "employees.employees_code", "first_name": name, "position.pos_name": "p.pos_name",
		"department.dept_name": "d.dept_name", "companies.name": "c.name", "is_active": "employees.is_active",
	}, "employees.id", "employees.id")
	if err != nil {
		return Page[models.Employees]{}, err
	}
	query := r.scoped(scope).Model(&models.Employees{}).Omit("Password").
		Joins("LEFT JOIN department d ON d.id = employees.dept_id AND d.deleted_at = 0").
		Joins("LEFT JOIN position p ON p.id = employees.pos_id AND p.deleted_at = 0").
		Joins("LEFT JOIN companies c ON c.id = employees.companies_id AND c.deleted_at = 0")
	if q.Search != "" {
		pattern := searchPattern(q.Search)
		query = query.Where("("+name+" ILIKE ? OR employees.employees_code ILIKE ? OR p.pos_name ILIKE ? OR d.dept_name ILIKE ? OR c.name ILIKE ? OR (CASE WHEN employees.is_active THEN 'active' ELSE 'inactive' END) ILIKE ?)", pattern, pattern, pattern, pattern, pattern, pattern)
	}
	return readPage[models.Employees](query.Preload("Department").Preload("Position").Preload("Companies"), q, order)
}

func (r *gormEmployeesRepository) FindScoped(scope AccessScope, id uint) (models.Employees, error) {
	var employee models.Employees
	if err := r.scoped(scope).First(&employee, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Employees{}, ErrEmployeeNotFound
		}
		return models.Employees{}, err
	}
	return employee, nil
}

// FindBriefScoped loads only what a cross-domain company check needs, so callers linking a record to an employee never drag the password hash around.
func (r *gormEmployeesRepository) FindBriefScoped(scope AccessScope, id uint) (models.Employees, error) {
	var employee models.Employees
	// The name and code travel with the scoping columns because the bulk raise has to name the people it is
	// blocking, and asking again per employee would be a second query for what the first one already read.
	// The selection stays explicit so the password hash never rides along.
	if err := r.scoped(scope).Select("id, companies_id, employees_code, first_name, last_name, is_active").First(&employee, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Employees{}, ErrEmployeeNotFound
		}
		return models.Employees{}, err
	}
	return employee, nil
}

var ErrEmployeeReferenceCompany = apperr.Validation("Employee department, position and employment type must belong to the employee company")

func (r *gormEmployeesRepository) ValidateCompanyReferences(employee models.Employees) error {
	checks := []struct {
		model any
		id    uint
	}{{&models.Department{}, employee.DeptID}, {&models.Position{}, employee.PosID}}
	if employee.EmploymentTypeID != nil {
		checks = append(checks, struct {
			model any
			id    uint
		}{&models.EmploymentType{}, *employee.EmploymentTypeID})
	}
	for _, check := range checks {
		if check.id == 0 {
			continue
		}
		var count int64
		if err := r.db.Model(check.model).Where("id = ? AND companies_id = ?", check.id, employee.CompaniesID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrEmployeeReferenceCompany
		}
	}
	return nil
}

func (r *gormEmployeesRepository) FindRole(roleID uint) (models.Roles, error) {
	var role models.Roles
	if err := r.db.First(&role, roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Roles{}, ErrRoleNotFound
		}
		return models.Roles{}, err
	}
	return role, nil
}

func (r *gormEmployeesRepository) Create(employee *models.Employees) error {
	// A non-nil pointer keeps an explicit false from becoming GORM's default true.
	// Keep the persisted schema/default unchanged for seeds and other DB clients.
	active := employee.IsActive
	row := struct {
		models.Employees `gorm:"embedded"`
		IsActive         *bool `gorm:"column:is_active"`
	}{Employees: *employee, IsActive: &active}
	if err := r.db.Table("employees").Create(&row).Error; err != nil {
		return err
	}
	*employee = row.Employees
	employee.IsActive = active
	return nil
}

func (r *gormEmployeesRepository) UpdateFields(employee *models.Employees, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	// Employee status changes serialize with payroll runs and cycle adjustments.
	if _, changingStatus := fields["is_active"]; changingStatus {
		if err := lockEmployeeCompanyCycles(r.db, employee.CompaniesID); err != nil {
			return err
		}
	}
	// Translate the layer-neutral ClearField marker into a SQL NULL here, so the service never has to name a GORM type.
	patch := make(map[string]any, len(fields))
	for key, value := range fields {
		if value == ClearField {
			patch[key] = gorm.Expr("NULL")
			continue
		}
		patch[key] = value
	}
	return r.db.Model(employee).Updates(patch).Error
}

// Delete retires the employee and the salary records that belong to them, in one transaction.
// A salary record outlives nothing: it exists to say what this person was paid, so leaving it behind when the
// person is gone leaves a row no page can reach and no rule can maintain. `is_active` is cleared with the same
// statement, since a deleted record that still reads as current is the state every write path refuses to touch.
func (r *gormEmployeesRepository) Delete(employee *models.Employees) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockEmployeeCompanyCycles(tx, employee.CompaniesID); err != nil {
			return err
		}
		// Match salary writes: cycles -> employee -> salary rows. Retired employees
		// can still be deleted; this lock deliberately does not require active status.
		var locked models.Employees
		if err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, employee.ID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrEmployeeNotFound
			}
			return err
		}
		if err := tx.Model(&models.SalariesStructure{}).
			Where("employees_id = ?", employee.ID).
			Updates(map[string]any{"is_active": false, "deleted_at": time.Now().UnixNano()}).Error; err != nil {
			return err
		}
		return tx.Delete(employee).Error
	})
}

func lockEmployeeCompanyCycles(tx *gorm.DB, companyID uint) error {
	var ids []uint
	return tx.Model(&models.PayrollCycle{}).Clauses(clause.Locking{Strength: "SHARE"}).
		Where("companies_id = ?", companyID).Order("id").Pluck("id", &ids).Error
}

func (r *gormEmployeesRepository) FindDetail(id uint) (models.Employees, error) {
	var employee models.Employees
	err := r.db.
		Preload("Department").
		Preload("Position").
		Preload("SalariesStructure", currentSalaryCondition).
		Preload("Bank").
		First(&employee, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Employees{}, ErrEmployeeNotFound
		}
		return models.Employees{}, err
	}
	return employee, nil
}

func (r *gormEmployeesRepository) AppendAuditLog(entry *models.AuditLog) error {
	return r.db.Create(entry).Error
}

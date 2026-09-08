package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
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
	ListScoped(scope AccessScope) ([]models.Employees, error)
	FindScoped(scope AccessScope, id uint) (models.Employees, error)
	FindBriefScoped(scope AccessScope, id uint) (models.Employees, error)
	FindRole(roleID uint) (models.Roles, error)
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

func (r *gormEmployeesRepository) ListScoped(scope AccessScope) ([]models.Employees, error) {
	var employees []models.Employees
	err := r.scoped(scope).
		Preload("Department").
		Preload("Position").
		Preload("Companies").
		Preload("Roles").
		Preload("SalariesStructure", currentSalaryCondition).
		Preload("Bank").
		Preload("EmploymentType").
		Find(&employees).Error
	return employees, err
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
	if err := r.scoped(scope).Select("id, companies_id, employees_code, first_name, last_name").First(&employee, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Employees{}, ErrEmployeeNotFound
		}
		return models.Employees{}, err
	}
	return employee, nil
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
	return r.db.Create(employee).Error
}

func (r *gormEmployeesRepository) UpdateFields(employee *models.Employees, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
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
		if err := tx.Model(&models.SalariesStructure{}).
			Where("employees_id = ?", employee.ID).
			Updates(map[string]any{"is_active": false, "deleted_at": time.Now().UnixNano()}).Error; err != nil {
			return err
		}
		return tx.Delete(employee).Error
	})
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

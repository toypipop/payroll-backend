package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Cycle-deduction lookup failures.
var (
	ErrCycleDeductionNotFound     = apperr.NotFound("Record not found or access denied")
	ErrCycleDeductionNotDeletable = apperr.NotFound("Record not found or unauthorized")
)

// CycleDeductionRepository is the persistence boundary for one-off deductions attached to a cycle.
type CycleDeductionRepository interface {
	WithinTx(func(CycleDeductionRepository, PayrollRunRepository) error) error
	ListScopedByCycle(scope AccessScope, cycleID uint, q PageQuery) (Page[models.PayrollCycleDeduction], error)
	FindScoped(scope AccessScope, id uint) (models.PayrollCycleDeduction, error)
	FindDetail(id uint) (models.PayrollCycleDeduction, error)
	Create(item *models.PayrollCycleDeduction, audit *models.AuditLog) error
	UpdateFields(item *models.PayrollCycleDeduction, fields map[string]any, audit *models.AuditLog) error
	Delete(item *models.PayrollCycleDeduction, audit *models.AuditLog) error
}

type gormCycleDeductionRepository struct {
	db *gorm.DB
}

// NewCycleDeductionRepository returns the GORM-backed implementation on the given connection.
func NewCycleDeductionRepository(db *gorm.DB) CycleDeductionRepository {
	return &gormCycleDeductionRepository{db: db}
}

func (r *gormCycleDeductionRepository) WithinTx(fn func(CycleDeductionRepository, PayrollRunRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(&gormCycleDeductionRepository{db: tx}, NewPayrollRunRepository(tx))
	})
}

// scoped filters by company and by employee: the table has a real employee column and CycleDeductionsPage is declared employee-level, so Own and Department bite.
func (r *gormCycleDeductionRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyEmployee("payroll_cycle_deduction.companies_id", "payroll_cycle_deduction.employees_id"))
}

func (r *gormCycleDeductionRepository) ListScopedByCycle(scope AccessScope, cycleID uint, q PageQuery) (Page[models.PayrollCycleDeduction], error) {
	query := r.scoped(scope).Where("payroll_cycle_deduction.cycle_id = ?", cycleID)
	return readEmployeeAdjustmentPage[models.PayrollCycleDeduction](query, q, "payroll_cycle_deduction", "deduction_type", "DeductionType")
}

func (r *gormCycleDeductionRepository) FindScoped(scope AccessScope, id uint) (models.PayrollCycleDeduction, error) {
	var item models.PayrollCycleDeduction
	if err := r.scoped(scope).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycleDeduction{}, ErrCycleDeductionNotFound
		}
		return models.PayrollCycleDeduction{}, err
	}
	return item, nil
}

// FindDetail reloads a row with its relations for the response body.
func (r *gormCycleDeductionRepository) FindDetail(id uint) (models.PayrollCycleDeduction, error) {
	var item models.PayrollCycleDeduction
	if err := r.db.Preload("DeductionType").Preload("Employees").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.PayrollCycleDeduction{}, ErrCycleDeductionNotFound
		}
		return models.PayrollCycleDeduction{}, err
	}
	return item, nil
}

func (r *gormCycleDeductionRepository) Create(item *models.PayrollCycleDeduction, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockActiveEmployees(tx, item.EmployeesID); err != nil {
			return err
		}
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = item.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormCycleDeductionRepository) UpdateFields(item *models.PayrollCycleDeduction, fields map[string]any, audit *models.AuditLog) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockEmployeeUpdate(tx, item.EmployeesID, fields, "employees_id"); err != nil {
			return err
		}
		if err := tx.Model(item).Updates(fields).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

func (r *gormCycleDeductionRepository) Delete(item *models.PayrollCycleDeduction, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(item).Error; err != nil {
			return err
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

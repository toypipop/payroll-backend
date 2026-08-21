package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// Salary lookup failures.
var (
	ErrSalaryRecordNotFound = apperr.NotFound("Salary record not found")
	ErrSalaryEmployeeDenied = apperr.NotFound("Employee not found or access denied")
)

// ArchiveDecision says what to do with a salary record that is no longer current.
type ArchiveDecision struct {
	ID   uint
	Keep bool
}

// SalariesStructureRepository is the persistence boundary for the salary-structure domain.
type SalariesStructureRepository interface {
	ListEmployeesWithSalary(scope AccessScope) ([]models.Employees, error)
	ListRecordsForEmployee(employeesID uint) ([]models.SalariesStructure, error)
	ListAllRecordsUnscoped(employeesID uint, companyID uint) ([]models.SalariesStructure, error)
	ClosedCycleUsage(recordIDs []uint) (map[uint]int64, error)
	FindRecord(id uint) (models.SalariesStructure, error)
	CountClosedCyclesCovering(companyID uint, date time.Time) (int64, error)
	ArchiveRecords(decisions []ArchiveDecision) error
	ArchiveAndCreate(decisions []ArchiveDecision, next *models.SalariesStructure) error
	DeleteRecord(record *models.SalariesStructure) error
	ActivateLatest(employeesID uint) error
}

type gormSalariesStructureRepository struct {
	db *gorm.DB
}

// NewSalariesStructureRepository returns the GORM-backed implementation on the given connection.
func NewSalariesStructureRepository(db *gorm.DB) SalariesStructureRepository {
	return &gormSalariesStructureRepository{db: db}
}

// ListEmployeesWithSalary lists the employees in scope with their current salary attached, so the page can show who still has none.
func (r *gormSalariesStructureRepository) ListEmployeesWithSalary(scope AccessScope) ([]models.Employees, error) {
	var employees []models.Employees
	err := r.db.Scopes(scope.ApplyEmployee("employees.companies_id", "employees.id")).
		Preload("Companies").
		Preload("Department").
		Preload("Position").
		Preload("SalariesStructure", "is_active = ?", true).
		Find(&employees).Error
	return employees, err
}

// ListRecordsForEmployee returns the records that count as history: the current one plus those preserved for audit.
func (r *gormSalariesStructureRepository) ListRecordsForEmployee(employeesID uint) ([]models.SalariesStructure, error) {
	var records []models.SalariesStructure
	err := r.db.Where("employees_id = ?", employeesID).
		Order("effective_date DESC, id DESC").
		Find(&records).Error
	return records, err
}

// ListAllRecordsUnscoped includes erased records, because the chronology rule weighs every effective date ever used.
func (r *gormSalariesStructureRepository) ListAllRecordsUnscoped(employeesID uint, companyID uint) ([]models.SalariesStructure, error) {
	var records []models.SalariesStructure
	err := r.db.Unscoped().
		Where("employees_id = ? AND companies_id = ?", employeesID, companyID).
		Order("id ASC").
		Find(&records).Error
	return records, err
}

// ClosedCycleUsage counts, per salary record, the payroll transactions belonging to a closed cycle.
func (r *gormSalariesStructureRepository) ClosedCycleUsage(recordIDs []uint) (map[uint]int64, error) {
	usage := make(map[uint]int64, len(recordIDs))
	if len(recordIDs) == 0 {
		return usage, nil
	}

	var rows []struct {
		SalariesStructureID uint
		Total               int64
	}
	err := r.db.Model(&models.PayrollTransaction{}).
		Select("payroll_transaction.salaries_structure_id, COUNT(*) AS total").
		Joins("JOIN payroll_cycle ON payroll_cycle.id = payroll_transaction.cycle_id").
		Where("payroll_transaction.salaries_structure_id IN ? AND payroll_cycle.is_closed = ?", recordIDs, true).
		Group("payroll_transaction.salaries_structure_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		usage[row.SalariesStructureID] = row.Total
	}
	return usage, nil
}

func (r *gormSalariesStructureRepository) FindRecord(id uint) (models.SalariesStructure, error) {
	var record models.SalariesStructure
	if err := r.db.First(&record, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.SalariesStructure{}, ErrSalaryRecordNotFound
		}
		return models.SalariesStructure{}, err
	}
	return record, nil
}

// CountClosedCyclesCovering reports how many closed cycles of the company contain the given date, which is what blocks backdating into an already-paid period.
func (r *gormSalariesStructureRepository) CountClosedCyclesCovering(companyID uint, date time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.PayrollCycle{}).
		Where("companies_id = ? AND is_closed = ? AND start_date <= ? AND end_date >= ?", companyID, true, date, date).
		Count(&count).Error
	return count, err
}

// applyDecisions retires records inside whichever transaction or session it is given.
func applyDecisions(db *gorm.DB, decisions []ArchiveDecision) error {
	for _, decision := range decisions {
		update := map[string]any{"deleted_at": time.Now().UnixNano()}
		if decision.Keep {
			update = map[string]any{"is_active": false}
		}
		if err := db.Model(&models.SalariesStructure{}).
			Where("id = ?", decision.ID).
			Updates(update).Error; err != nil {
			return err
		}
	}
	return nil
}

// ArchiveRecords retires records without adding a replacement, for the disable path.
func (r *gormSalariesStructureRepository) ArchiveRecords(decisions []ArchiveDecision) error {
	if len(decisions) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		return applyDecisions(tx, decisions)
	})
}

// ArchiveAndCreate retires the outgoing records and inserts the new one as one transaction, so an employee is never left without an active salary.
func (r *gormSalariesStructureRepository) ArchiveAndCreate(decisions []ArchiveDecision, next *models.SalariesStructure) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := applyDecisions(tx, decisions); err != nil {
			return err
		}
		return tx.Create(next).Error
	})
}

func (r *gormSalariesStructureRepository) DeleteRecord(record *models.SalariesStructure) error {
	return r.db.Delete(record).Error
}

// ActivateLatest promotes the employee's most recent surviving record to current.
func (r *gormSalariesStructureRepository) ActivateLatest(employeesID uint) error {
	var previous models.SalariesStructure
	err := r.db.Where("employees_id = ?", employeesID).
		Order("effective_date DESC, id DESC").
		First(&previous).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return r.db.Model(&models.SalariesStructure{}).
		Where("id = ?", previous.ID).
		Update("is_active", true).Error
}

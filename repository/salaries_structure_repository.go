package repository

import (
	"encoding/json"
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Salary lookup failures.
var (
	ErrSalaryRecordNotFound = apperr.NotFound("Salary record not found")
	ErrSalaryEmployeeDenied = apperr.NotFound("Employee not found or access denied")
)

// ErrSalaryVersionConflict is what a caller gets when the record moved between the read and the write.
var ErrSalaryVersionConflict = apperr.Precondition("This salary record was changed by someone else. Reload and try again.")

// SalariesStructureRepository is the persistence boundary for the salary-structure domain.
type SalariesStructureRepository interface {
	// WithinTx runs fn against a repository bound to a single database transaction.
	WithinTx(fn func(SalariesStructureRepository) error) error
	// LockCompanyCycles takes a share lock on the company's payroll cycles for the rest of the transaction, so a cycle cannot be closed underneath a check that just read its state.
	LockCompanyCycles(companyID uint) error

	ListEmployeesWithSalary(scope AccessScope) ([]models.Employees, error)
	ListRecordsForEmployee(employeesID uint) ([]models.SalariesStructure, error)
	ListAllRecordsUnscoped(employeesID uint, companyID uint) ([]models.SalariesStructure, error)
	ClosedCyclesUsing(recordIDs []uint) (map[uint][]string, error)
	FindRecord(id uint) (models.SalariesStructure, error)
	CountClosedCyclesCovering(companyID uint, date time.Time) (int64, error)
	CreateRecord(next *models.SalariesStructure) error
	UpdateAmount(record models.SalariesStructure, amount float64, switchOn bool) error
	MoveRecord(record models.SalariesStructure, effectiveDate time.Time, amount float64) error
	SwitchOffAll(employeesID uint) error
	DeleteRecord(record *models.SalariesStructure) error

	// AppendAuditLog records one change. It is called inside the same transaction as the write it describes,
	// so a change that cannot be recorded is not made.
	AppendAuditLog(entry *models.AuditLog) error
	// ListAuditForEmployee returns that employee's trail, newest first.
	ListAuditForEmployee(employeesID uint) ([]models.SalaryAudit, error)
}

type gormSalariesStructureRepository struct {
	db *gorm.DB
}

// NewSalariesStructureRepository returns the GORM-backed implementation on the given connection.
func NewSalariesStructureRepository(db *gorm.DB) SalariesStructureRepository {
	return &gormSalariesStructureRepository{db: db}
}

// WithinTx runs fn against a repository bound to one transaction; any error rolls the whole thing back.
func (r *gormSalariesStructureRepository) WithinTx(fn func(SalariesStructureRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(&gormSalariesStructureRepository{db: tx})
	})
}

// LockCompanyCycles locks the company's cycles in share mode, which conflicts with the FOR UPDATE a payroll close takes.
// A salary rule that asks "is this date inside a closed cycle" is only meaningful if the answer cannot change between the check and the write; without this the check passes, a concurrent close commits, and the new record lands inside a cycle someone has already been paid against.
func (r *gormSalariesStructureRepository) LockCompanyCycles(companyID uint) error {
	var ids []uint
	return r.db.Model(&models.PayrollCycle{}).
		Clauses(clause.Locking{Strength: "SHARE"}).
		Where("companies_id = ?", companyID).
		Pluck("id", &ids).Error
}

// ListEmployeesWithSalary lists the employees in scope with their current salary attached, so the page can show who still has none.
func (r *gormSalariesStructureRepository) ListEmployeesWithSalary(scope AccessScope) ([]models.Employees, error) {
	var employees []models.Employees
	err := r.db.Scopes(scope.ApplyEmployee("employees.companies_id", "employees.id")).
		Preload("Companies").
		Preload("Department").
		Preload("Position").
		Preload("SalariesStructure", currentSalaryCondition).
		Find(&employees).Error
	return employees, err
}

// currentSalaryCondition selects the record in force: the one with the latest effective date among the records
// that are switched on. It is a condition rather than a stored flag because a flag has to be recalculated every
// time a record is added, corrected, moved or deleted — and the moment it is not, the page shows a salary the
// payroll run does not use.
const currentSalaryCondition = `is_active = true AND effective_date = (
	SELECT MAX(latest.effective_date) FROM salaries_structure AS latest
	WHERE latest.employees_id = salaries_structure.employees_id
	  AND latest.deleted_at = 0
	  AND latest.is_active = true
)`

// ListRecordsForEmployee returns the records that count as history: the current one plus those preserved for audit.
func (r *gormSalariesStructureRepository) ListRecordsForEmployee(employeesID uint) ([]models.SalariesStructure, error) {
	var records []models.SalariesStructure
	err := r.db.Where("employees_id = ?", employeesID).
		Order("effective_date DESC, id DESC").
		Find(&records).Error
	return records, err
}

// ListAllRecordsUnscoped includes deleted records. The service filters them out of both things it decides —
// which record is current, and which effective date the history has reached — so what the deleted rows are here
// for is to be visible as deleted rather than silently absent.
func (r *gormSalariesStructureRepository) ListAllRecordsUnscoped(employeesID uint, companyID uint) ([]models.SalariesStructure, error) {
	var records []models.SalariesStructure
	err := r.db.Unscoped().
		Where("employees_id = ? AND companies_id = ?", employeesID, companyID).
		Order("id ASC").
		Find(&records).Error
	return records, err
}

// ClosedCyclesUsing names, per salary record, the closed payroll cycles that were calculated from it.
// A record that appears here was the basis of a payslip someone has already been paid, which is what makes
// it read-only — and naming the cycles is what lets the screen say why rather than just that.
func (r *gormSalariesStructureRepository) ClosedCyclesUsing(recordIDs []uint) (map[uint][]string, error) {
	usage := make(map[uint][]string, len(recordIDs))
	if len(recordIDs) == 0 {
		return usage, nil
	}

	var rows []struct {
		SalariesStructureID uint
		CycleName           string
	}
	err := r.db.Model(&models.PayrollTransaction{}).
		Select("DISTINCT payroll_transaction.salaries_structure_id, payroll_cycle.cycle_name, payroll_cycle.start_date").
		Joins("JOIN payroll_cycle ON payroll_cycle.id = payroll_transaction.cycle_id").
		Where("payroll_transaction.salaries_structure_id IN ? AND payroll_cycle.is_closed = ?", recordIDs, true).
		Order("payroll_cycle.start_date").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		usage[row.SalariesStructureID] = append(usage[row.SalariesStructureID], row.CycleName)
	}
	return usage, nil
}

// AppendAuditLog writes one trail entry. Rows are append-only, so this is the only method that touches the table.
func (r *gormSalariesStructureRepository) AppendAuditLog(entry *models.AuditLog) error {
	return r.db.Create(entry).Error
}

// ListAuditForEmployee keeps the existing salary-history response stable while the storage is the central
// audit_log table. The UI therefore remains compatible while all new salary writes use the shared trail.
func (r *gormSalariesStructureRepository) ListAuditForEmployee(employeesID uint) ([]models.SalaryAudit, error) {
	var rows []models.AuditLog
	err := r.db.Where("entity_type = ? AND entity_id = ?", "employee", employeesID).
		Where("event_type IN ?", []models.AuditEventType{
			models.AuditEventSalaryCreated, models.AuditEventSalaryChanged, models.AuditEventSalaryMoved,
			models.AuditEventSalaryDeactivated, models.AuditEventSalaryDeleted,
		}).Order("occurred_at DESC, id DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	entries := make([]models.SalaryAudit, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, salaryAuditFromLog(row))
	}
	return entries, nil
}

func salaryAuditFromLog(entry models.AuditLog) models.SalaryAudit {
	audit := models.SalaryAudit{ID: entry.ID, CreatedAt: entry.OccurredAt, Reason: entry.Reason, ActorName: entry.ActorNameSnapshot}
	if entry.CompaniesID != nil {
		audit.CompaniesID = *entry.CompaniesID
	}
	if entry.EntityID != nil {
		audit.EmployeesID = *entry.EntityID
	}
	if entry.ActorUserID != nil {
		audit.ActorID = *entry.ActorUserID
	}
	switch entry.EventType {
	case models.AuditEventSalaryCreated:
		audit.Action = models.SalaryAuditCreated
	case models.AuditEventSalaryChanged:
		audit.Action = models.SalaryAuditAmountChanged
	case models.AuditEventSalaryMoved:
		audit.Action = models.SalaryAuditMoved
	case models.AuditEventSalaryDeactivated:
		audit.Action = models.SalaryAuditSwitchedOff
	case models.AuditEventSalaryDeleted:
		audit.Action = models.SalaryAuditDeleted
	}
	decodeSalaryAuditData(entry.BeforeData, &audit, true)
	decodeSalaryAuditData(entry.AfterData, &audit, false)
	return audit
}

func decodeSalaryAuditData(data json.RawMessage, audit *models.SalaryAudit, before bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return
	}
	if raw, ok := fields["salary_record_id"]; ok {
		var recordID uint
		if json.Unmarshal(raw, &recordID) == nil {
			audit.SalariesStructureID = &recordID
		}
	}
	if raw, ok := fields["base_salary"]; ok {
		var amount float64
		if json.Unmarshal(raw, &amount) == nil {
			if before {
				audit.FromAmount = &amount
			} else {
				audit.ToAmount = &amount
			}
		}
	}
	if raw, ok := fields["effective_date"]; ok {
		var value string
		if json.Unmarshal(raw, &value) == nil {
			if date, err := time.Parse("2006-01-02", value); err == nil {
				if before {
					audit.FromDate = &date
				} else {
					audit.ToDate = &date
				}
			}
		}
	}
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

// CreateRecord adds a record to the employee's timeline. Nothing else moves: which record is in force is read
// from the effective dates, so adding one never means editing another.
func (r *gormSalariesStructureRepository) CreateRecord(next *models.SalariesStructure) error {
	return r.db.Create(next).Error
}

// UpdateAmount corrects the figure on a record, for a save that reuses that record's effective date. Same date
// means the same step in the salary's history, so the figure is fixed in place rather than adding a step.
//
// switchOn says whether the correction also brings the record back into use. Only the record with the latest
// effective date may do that: switching on an older one while a later record sits switched off would quietly make
// the older figure the salary in force, which is not what correcting a past period asks for.
//
// The version in the WHERE is the whole guard. A second writer that read the same record before this one
// committed finds the version has moved and gets a conflict instead of overwriting the correction.
func (r *gormSalariesStructureRepository) UpdateAmount(record models.SalariesStructure, amount float64, switchOn bool) error {
	update := map[string]any{"base_salaries": amount, "version": record.Version + 1}
	if switchOn {
		update["is_active"] = true
	}
	result := r.db.Model(&models.SalariesStructure{}).
		Where("id = ? AND version = ?", record.ID, record.Version).
		Updates(update)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSalaryVersionConflict
	}
	return nil
}

// MoveRecord changes where a record sits on the timeline, and what it pays while it is there. Moving a record
// re-prices two periods at once — the one it leaves and the one it lands in — which is why the caller checks the
// closed cycles on both sides before calling this.
//
// It does not touch `is_active`: moving a record says when its figure applies, not whether it is in use.
func (r *gormSalariesStructureRepository) MoveRecord(record models.SalariesStructure, effectiveDate time.Time, amount float64) error {
	result := r.db.Model(&models.SalariesStructure{}).
		Where("id = ? AND version = ?", record.ID, record.Version).
		Updates(map[string]any{
			"effective_date": effectiveDate,
			"base_salaries":  amount,
			"version":        record.Version + 1,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSalaryVersionConflict
	}
	return nil
}

// SwitchOffAll turns every one of the employee's records off, which is what disabling a salary means: no record
// counts any more, so a run finds nothing to pay them from. Turning one back on is a save on that record's date.
func (r *gormSalariesStructureRepository) SwitchOffAll(employeesID uint) error {
	return r.db.Model(&models.SalariesStructure{}).
		Where("employees_id = ? AND is_active = ?", employeesID, true).
		Update("is_active", false).Error
}

// DeleteRecord erases the record only while it still carries the version the caller read.
// Nothing is promoted afterwards: with the timeline as the source of truth, the record before it becomes the
// latest one by simply still being there.
func (r *gormSalariesStructureRepository) DeleteRecord(record *models.SalariesStructure) error {
	result := r.db.Where("version = ?", record.Version).Delete(record)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSalaryVersionConflict
	}
	return nil
}

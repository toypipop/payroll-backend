package service

import (
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// salaryDateLayout is the only date shape the API accepts for an effective date.
const salaryDateLayout = "2006-01-02"

// Salary rule failures.
var (
	ErrSalaryDateInvalid    = apperr.Validation("Invalid date format. Expected YYYY-MM-DD")
	ErrSalaryEmployeeTarget = apperr.NotFound("Employees not found or access denied")
	ErrSalaryDateInClosed   = apperr.Validation("Effective date falls within an already-closed payroll cycle and cannot be used")
	ErrSalaryDateBackward   = apperr.Validation("Effective date cannot be before the most recent salary record's effective date")
	ErrSalaryCompanyDenied  = apperr.Forbidden("Access denied")
	ErrSalaryRecordInUse    = apperr.Validation("Cannot delete — this record was already used in a closed payroll cycle's calculation. Disable it instead if it should no longer apply.")
)

// Re-exported so handlers import a single package.
var (
	ErrSalaryRecordNotFound = repository.ErrSalaryRecordNotFound
	ErrSalaryEmployeeDenied = repository.ErrSalaryEmployeeDenied
)

// SalaryInput is what a caller may set when saving a salary structure.
type SalaryInput struct {
	EmployeesID   uint
	CompaniesID   uint
	BaseSalaries  float64
	EffectiveDate string
	Disable       bool
}

// SalaryHistoryEntry is one row of an employee's salary history plus whether it can still be erased.
type SalaryHistoryEntry struct {
	Record    models.SalariesStructure
	CanDelete bool
}

// SaveResult reports which branch a save took, since disabling and replacing return different messages.
type SaveResult struct {
	Disabled bool
}

// SalariesStructureService owns the salary-history rules.
type SalariesStructureService struct {
	repo      repository.SalariesStructureRepository
	employees repository.EmployeesRepository
}

// NewSalariesStructureService wires the service to repository implementations.
func NewSalariesStructureService(
	repo repository.SalariesStructureRepository,
	employees repository.EmployeesRepository,
) *SalariesStructureService {
	return &SalariesStructureService{repo: repo, employees: employees}
}

// ListEmployees returns the employees in scope with their current salary attached.
func (s *SalariesStructureService) ListEmployees(scope repository.AccessScope) ([]models.Employees, error) {
	return s.repo.ListEmployeesWithSalary(scope)
}

// History returns every record that counts as history for one employee, flagging which ones are still safe to erase.
func (s *SalariesStructureService) History(scope repository.AccessScope, employeesID uint) ([]SalaryHistoryEntry, error) {
	if _, err := s.employees.FindBriefScoped(scope, employeesID); err != nil {
		return nil, ErrSalaryEmployeeDenied
	}

	records, err := s.repo.ListRecordsForEmployee(employeesID)
	if err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	usage, err := s.repo.ClosedCycleUsage(ids)
	if err != nil {
		return nil, err
	}

	entries := make([]SalaryHistoryEntry, 0, len(records))
	for _, record := range records {
		entries = append(entries, SalaryHistoryEntry{Record: record, CanDelete: usage[record.ID] == 0})
	}
	return entries, nil
}

// decisionsFor turns usage counts into per-record archive decisions: a record a closed cycle relied on is kept for audit, anything else is erased.
func decisionsFor(records []models.SalariesStructure, usage map[uint]int64) []repository.ArchiveDecision {
	decisions := make([]repository.ArchiveDecision, 0, len(records))
	for _, record := range records {
		decisions = append(decisions, repository.ArchiveDecision{ID: record.ID, Keep: usage[record.ID] > 0})
	}
	return decisions
}

// Save either disables an employee's current salary or supersedes it with a new one.
func (s *SalariesStructureService) Save(scope repository.AccessScope, input SalaryInput) (SaveResult, error) {
	if input.EmployeesID == 0 || input.CompaniesID == 0 {
		return SaveResult{}, ErrSalaryEmployeeTarget
	}

	effectiveDate, err := time.Parse(salaryDateLayout, input.EffectiveDate)
	if err != nil {
		return SaveResult{}, ErrSalaryDateInvalid
	}

	if _, ok := scope.ResolveCompanyID(input.CompaniesID); !ok {
		return SaveResult{}, ErrSalaryEmployeeTarget
	}
	employee, err := s.employees.FindBriefScoped(scope, input.EmployeesID)
	if err != nil || employee.CompaniesID != input.CompaniesID {
		return SaveResult{}, ErrSalaryEmployeeTarget
	}

	existing, err := s.repo.ListAllRecordsUnscoped(input.EmployeesID, employee.CompaniesID)
	if err != nil {
		return SaveResult{}, err
	}

	active := make([]models.SalariesStructure, 0, len(existing))
	for _, record := range existing {
		if record.IsActive {
			active = append(active, record)
		}
	}

	activeIDs := make([]uint, 0, len(active))
	for _, record := range active {
		activeIDs = append(activeIDs, record.ID)
	}
	usage, err := s.repo.ClosedCycleUsage(activeIDs)
	if err != nil {
		return SaveResult{}, err
	}

	if input.Disable {
		if err := s.repo.ArchiveRecords(decisionsFor(active, usage)); err != nil {
			return SaveResult{}, err
		}
		return SaveResult{Disabled: true}, nil
	}

	// Backdating into a period that has already been paid out and closed would change a figure someone has already been paid against.
	closedCycles, err := s.repo.CountClosedCyclesCovering(employee.CompaniesID, effectiveDate)
	if err != nil {
		return SaveResult{}, err
	}
	if closedCycles > 0 {
		return SaveResult{}, ErrSalaryDateInClosed
	}

	// The effective date must not move backward from the most recent record, erased ones included, so history stays in chronological order.
	if len(existing) > 0 {
		last := existing[len(existing)-1]
		if effectiveDate.Before(last.EffectiveDate) {
			return SaveResult{}, ErrSalaryDateBackward
		}
	}

	next := models.SalariesStructure{
		CompaniesID:   employee.CompaniesID,
		EmployeesID:   input.EmployeesID,
		BaseSalaries:  input.BaseSalaries,
		EffectiveDate: effectiveDate,
		IsActive:      true,
	}
	if err := s.repo.ArchiveAndCreate(decisionsFor(active, usage), &next); err != nil {
		return SaveResult{}, err
	}
	return SaveResult{}, nil
}

// DeleteRecord erases a salary record, but only one no closed cycle ever used.
func (s *SalariesStructureService) DeleteRecord(scope repository.AccessScope, id uint) error {
	record, err := s.repo.FindRecord(id)
	if err != nil {
		return err
	}
	if !scope.CanAccessCompany(record.CompaniesID) {
		return ErrSalaryCompanyDenied
	}

	usage, err := s.repo.ClosedCycleUsage([]uint{record.ID})
	if err != nil {
		return err
	}
	if usage[record.ID] > 0 {
		return ErrSalaryRecordInUse
	}

	if err := s.repo.DeleteRecord(&record); err != nil {
		return err
	}

	// Erasing the current record should roll the employee back to the state before that mistake, not leave them with no active salary at all.
	if record.IsActive {
		return s.repo.ActivateLatest(record.EmployeesID)
	}
	return nil
}

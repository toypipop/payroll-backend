package service

import (
	"strings"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// Salary rule failures.
var (
	ErrSalaryDateInvalid    = apperr.Validation("Invalid date format. Expected YYYY-MM-DD")
	ErrSalaryEmployeeTarget = apperr.NotFound("Employees not found or access denied")
	// Both of these are the request contradicting stored state rather than malformed input, so they answer 409:
	// the same request would be accepted against a different salary history.
	ErrSalaryDateInClosed  = apperr.Conflict("Effective date falls within an already-closed payroll cycle and cannot be used")
	ErrSalaryCompanyDenied = apperr.Forbidden("Access denied")
	ErrSalaryRecordInUse   = apperr.Validation("Cannot delete — this record was already used in a closed payroll cycle's calculation. Disable it instead if it should no longer apply.")
	// One effective date holds one record, so a move onto a date already taken has to say so rather than fail on the index.
	ErrSalaryDateTaken = apperr.Conflict("Another salary record already uses that effective date. Delete it first, or pick another date.")
	// Correcting a record a closed cycle was calculated from would rewrite the basis of a payslip already paid out.
	ErrSalaryRecordPaid = apperr.Conflict("This effective date was already used in a closed payroll cycle's calculation and cannot be changed. Add the new amount on a later effective date instead.")
)

// Re-exported so a handler maps one package's errors.
var ErrSalaryVersionConflict = repository.ErrSalaryVersionConflict

// ErrSalaryReasonRequired is returned when a write arrives with nothing said about why.
var ErrSalaryReasonRequired = apperr.Validation("A reason is required for every salary change")

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
	// Version is what the form was seeded from. Zero means the client sent none, which is how a create behaves.
	Version uint
	// Reason is why the change is being made. It is required: an audit trail of unexplained changes answers
	// only half the question it exists for.
	Reason string
}

// SalaryRecordEdit is what a caller may change on a record that already exists: where it sits on the timeline and
// what it pays while it is there.
type SalaryRecordEdit struct {
	EffectiveDate string
	BaseSalaries  float64
	// Version is the record's own version as the row was read; the edit is refused if it has moved since.
	Version uint
	// Reason is why the record is being moved. Required, like every other salary write.
	Reason string
}

// SalaryHistoryEntry is one row of an employee's salary history, plus the closed cycles that were paid from
// it. Those names are the whole reason a row is read-only, so CanDelete is derived from them rather than
// tracked beside them.
type SalaryHistoryEntry struct {
	Record    models.SalariesStructure
	LockedBy  []string
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
	usage, err := s.repo.ClosedCyclesUsing(ids)
	if err != nil {
		return nil, err
	}

	entries := make([]SalaryHistoryEntry, 0, len(records))
	for _, record := range records {
		lockedBy := usage[record.ID]
		entries = append(entries, SalaryHistoryEntry{Record: record, LockedBy: lockedBy, CanDelete: len(lockedBy) == 0})
	}
	return entries, nil
}

// AuditTrail returns who changed this employee's pay, when, and why — newest first.
func (s *SalariesStructureService) AuditTrail(scope repository.AccessScope, employeesID uint) ([]models.SalaryAudit, error) {
	if _, err := s.employees.FindBriefScoped(scope, employeesID); err != nil {
		return nil, ErrSalaryEmployeeDenied
	}
	return s.repo.ListAuditForEmployee(employeesID)
}

// currentRecord is the record in force: the one with the latest effective date among the records that are
// switched on. It is derived from the dates every time rather than marked with a flag, so it cannot fall out of
// step with the timeline the payroll run reads.
func currentRecord(records []models.SalariesStructure) (models.SalariesStructure, bool) {
	var current models.SalariesStructure
	found := false
	for _, record := range records {
		if record.DeletedAt != 0 || !record.IsActive {
			continue
		}
		if !found || record.EffectiveDate.After(current.EffectiveDate) {
			current, found = record, true
		}
	}
	return current, found
}

// currentVersion is the version the page was seeded with: the one carried by the record in force, or zero when
// the employee has no salary at all. It is what a save is checked against, so it has to be the number the page
// showed — not the highest number on the timeline, which a record added to the past would have moved.
func currentVersion(records []models.SalariesStructure) uint {
	if record, ok := currentRecord(records); ok {
		return record.Version
	}
	return 0
}

// nextVersion is the number a newly added record takes: one past the highest this employee has ever used, deleted
// records included. Counting from the record in force instead would hand out the same number twice — a record
// added to the past does not change what is in force, so the next one added would start from the same place.
func nextVersion(records []models.SalariesStructure) uint {
	var highest uint
	for _, record := range records {
		if record.Version > highest {
			highest = record.Version
		}
	}
	return highest + 1
}

// isLatestOn reports whether the given date is at or past every record still on the timeline, which is what makes
// a record the one in force once it is switched on.
func isLatestOn(records []models.SalariesStructure, date time.Time) bool {
	// An effective date is a calendar day, not an instant: comparing the raw values would let a row stored a
	// few hours off — which is what a database written under an older timezone convention holds — outrank a
	// row on the very same day.
	day := timeutil.DayStart(date)
	for _, record := range records {
		if record.DeletedAt == 0 && timeutil.DayStart(record.EffectiveDate).After(day) {
			return false
		}
	}
	return true
}

// auditActor names the person making the change, snapshotting the name so the trail keeps reading correctly
// after they are renamed or removed.
func auditActor(scope repository.AccessScope) (uint, string) {
	name := strings.TrimSpace(scope.User.FirstName + " " + scope.User.LastName)
	if name == "" {
		name = "Unknown"
	}
	return scope.User.ID, name
}

// recordAudit appends one trail entry for a change that has just been made in the same transaction.
func recordAudit(repo repository.SalariesStructureRepository, scope repository.AccessScope, entry models.SalaryAudit) error {
	entry.ActorID, entry.ActorName = auditActor(scope)
	return repo.AppendAudit(&entry)
}

// cleanReason is the reason a write may proceed with, or the error saying it may not.
func cleanReason(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrSalaryReasonRequired
	}
	return trimmed, nil
}

// salaryWrite is one employee's part of a salary change, already resolved and authorized. It exists so the
// single-employee save and the bulk raise share one set of rules rather than two copies that drift.
type salaryWrite struct {
	CompaniesID   uint
	EmployeesID   uint
	BaseSalaries  float64
	EffectiveDate time.Time
	Reason        string
	// Existing is the employee's whole timeline, read inside the same transaction as the write.
	Existing []models.SalariesStructure
}

// applySalaryOn puts one employee's new figure on the timeline and records why. It takes the repository
// rather than reaching for the service's own, so the caller decides the transaction: a save opens one per
// employee, a bulk raise opens one for the batch.
func applySalaryOn(repo repository.SalariesStructureRepository, scope repository.AccessScope, write salaryWrite) error {
	// Backdating into a period that has already been paid out and closed would change a figure someone has already been paid against.
	closedCycles, err := repo.CountClosedCyclesCovering(write.CompaniesID, write.EffectiveDate)
	if err != nil {
		return err
	}
	if closedCycles > 0 {
		return ErrSalaryDateInClosed
	}

	// One effective date is one step in the salary's history, so a save that reuses a date already on the
	// timeline corrects that record rather than adding a second one for the same day.
	for _, record := range write.Existing {
		if record.DeletedAt != 0 || timeutil.DateKey(record.EffectiveDate) != timeutil.DateKey(write.EffectiveDate) {
			continue
		}

		// Correcting the figure a closed cycle was calculated from would rewrite the basis of a payslip
		// already paid against it, so that record is fixed for good and the change belongs on a later date.
		usage, err := repo.ClosedCyclesUsing([]uint{record.ID})
		if err != nil {
			return err
		}
		if len(usage[record.ID]) > 0 {
			return ErrSalaryRecordPaid
		}
		// Correcting the record with the latest date brings it back into use — that is how a disabled salary
		// is switched back on. Correcting an earlier one only changes what that period pays.
		if err := repo.UpdateAmount(record, write.BaseSalaries, isLatestOn(write.Existing, write.EffectiveDate)); err != nil {
			return err
		}
		was, now, on := record.BaseSalaries, write.BaseSalaries, record.EffectiveDate
		return recordAudit(repo, scope, models.SalaryAudit{
			CompaniesID:         write.CompaniesID,
			EmployeesID:         write.EmployeesID,
			SalariesStructureID: &record.ID,
			Action:              models.SalaryAuditAmountChanged,
			Reason:              write.Reason,
			FromAmount:          &was,
			ToAmount:            &now,
			ToDate:              &on,
		})
	}

	// A record may be added anywhere on the timeline, not only after the last one: a run prices each cycle
	// from the record in force on that cycle's dates, so one slotted into the past is the rate that period is
	// meant to be paid at. Which record is in force now follows from the dates, so nothing else is touched.
	next := models.SalariesStructure{
		CompaniesID:   write.CompaniesID,
		EmployeesID:   write.EmployeesID,
		BaseSalaries:  write.BaseSalaries,
		EffectiveDate: write.EffectiveDate,
		IsActive:      true,
		Version:       nextVersion(write.Existing),
	}
	if err := repo.CreateRecord(&next); err != nil {
		return err
	}
	amount := next.BaseSalaries
	return recordAudit(repo, scope, models.SalaryAudit{
		CompaniesID:         write.CompaniesID,
		EmployeesID:         write.EmployeesID,
		SalariesStructureID: &next.ID,
		Action:              models.SalaryAuditCreated,
		Reason:              write.Reason,
		ToAmount:            &amount,
		ToDate:              &write.EffectiveDate,
	})
}

// Save either disables an employee's current salary or supersedes it with a new one.
func (s *SalariesStructureService) Save(scope repository.AccessScope, input SalaryInput) (SaveResult, error) {
	if input.EmployeesID == 0 || input.CompaniesID == 0 {
		return SaveResult{}, ErrSalaryEmployeeTarget
	}

	reason, err := cleanReason(input.Reason)
	if err != nil {
		return SaveResult{}, err
	}

	effectiveDate, err := timeutil.ParseDayStart(input.EffectiveDate)
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

	// Every rule below is a check the write depends on, so the reads, the checks and the write share one transaction — and the cycle lock holds the closed/open state still for its duration.
	result := SaveResult{}
	err = s.repo.WithinTx(func(repo repository.SalariesStructureRepository) error {
		if err := repo.LockCompanyCycles(employee.CompaniesID); err != nil {
			return err
		}

		existing, err := repo.ListAllRecordsUnscoped(input.EmployeesID, employee.CompaniesID)
		if err != nil {
			return err
		}

		// A form seeded from an older version of this employee's salary is refused outright, so the user is
		// told their screen is stale rather than having their edit silently replace whatever landed in between.
		if input.Version != currentVersion(existing) {
			return ErrSalaryVersionConflict
		}

		if input.Disable {
			if err := repo.SwitchOffAll(input.EmployeesID); err != nil {
				return err
			}
			if err := recordAudit(repo, scope, models.SalaryAudit{
				CompaniesID: employee.CompaniesID,
				EmployeesID: input.EmployeesID,
				Action:      models.SalaryAuditSwitchedOff,
				Reason:      reason,
			}); err != nil {
				return err
			}
			result = SaveResult{Disabled: true}
			return nil
		}

		return applySalaryOn(repo, scope, salaryWrite{
			CompaniesID:   employee.CompaniesID,
			EmployeesID:   input.EmployeesID,
			BaseSalaries:  input.BaseSalaries,
			EffectiveDate: effectiveDate,
			Reason:        reason,
			Existing:      existing,
		})
	})
	if err != nil {
		return SaveResult{}, err
	}
	return result, nil
}

// UpdateRecord moves a record along the timeline and re-prices it. It is the one path that changes an effective
// date: a save is addressed by date and so can only ever reach the record already sitting on that date.
//
// Moving a record changes what two periods pay — the one it leaves and the one it lands in — so both sides are
// checked against the cycles that have already been closed and paid out.
func (s *SalariesStructureService) UpdateRecord(scope repository.AccessScope, id uint, input SalaryRecordEdit) error {
	reason, err := cleanReason(input.Reason)
	if err != nil {
		return err
	}

	effectiveDate, err := timeutil.ParseDayStart(input.EffectiveDate)
	if err != nil {
		return ErrSalaryDateInvalid
	}

	record, err := s.repo.FindRecord(id)
	if err != nil {
		return err
	}
	if !scope.CanAccessCompany(record.CompaniesID) {
		return ErrSalaryCompanyDenied
	}

	return s.repo.WithinTx(func(repo repository.SalariesStructureRepository) error {
		if err := repo.LockCompanyCycles(record.CompaniesID); err != nil {
			return err
		}

		existing, err := repo.ListAllRecordsUnscoped(record.EmployeesID, record.CompaniesID)
		if err != nil {
			return err
		}

		current, found := recordByID(existing, id)
		if !found {
			return ErrSalaryRecordNotFound
		}
		// The version is compared, not merely checked for presence: an edit that omits it used to skip the
		// lock entirely and take last-write-wins, which is the exact thing this path exists to prevent. A
		// record whose stored version really is zero still matches a zero, so nothing old becomes unusable.
		if input.Version != current.Version {
			return ErrSalaryVersionConflict
		}

		// A record a closed cycle was calculated from is the basis of a payslip already paid against it: neither
		// the date it covers nor the figure it carries may move.
		usage, err := repo.ClosedCyclesUsing([]uint{current.ID})
		if err != nil {
			return err
		}
		if len(usage[current.ID]) > 0 {
			return ErrSalaryRecordPaid
		}

		// Landing inside a period that has been closed and paid would change a figure someone was already paid.
		closedCycles, err := repo.CountClosedCyclesCovering(record.CompaniesID, effectiveDate)
		if err != nil {
			return err
		}
		if closedCycles > 0 {
			return ErrSalaryDateInClosed
		}

		// One date holds one record.
		for _, other := range existing {
			if other.ID == current.ID || other.DeletedAt != 0 {
				continue
			}
			if timeutil.DateKey(other.EffectiveDate) == timeutil.DateKey(effectiveDate) {
				return ErrSalaryDateTaken
			}
		}

		if err := repo.MoveRecord(current, effectiveDate, input.BaseSalaries); err != nil {
			return err
		}
		wasAmount, wasDate := current.BaseSalaries, current.EffectiveDate
		nowAmount := input.BaseSalaries
		return recordAudit(repo, scope, models.SalaryAudit{
			CompaniesID:         current.CompaniesID,
			EmployeesID:         current.EmployeesID,
			SalariesStructureID: &current.ID,
			Action:              models.SalaryAuditMoved,
			Reason:              reason,
			FromAmount:          &wasAmount,
			ToAmount:            &nowAmount,
			FromDate:            &wasDate,
			ToDate:              &effectiveDate,
		})
	})
}

// recordByID finds one record in a list the caller has already read, so a check and the write that follows it read
// the same row rather than two separate queries with a gap between them.
func recordByID(records []models.SalariesStructure, id uint) (models.SalariesStructure, bool) {
	for _, record := range records {
		if record.ID == id && record.DeletedAt == 0 {
			return record, true
		}
	}
	return models.SalariesStructure{}, false
}

// DeleteRecord erases a salary record, but only one no closed cycle ever used.
func (s *SalariesStructureService) DeleteRecord(scope repository.AccessScope, id uint, reasonValue string) error {
	reason, err := cleanReason(reasonValue)
	if err != nil {
		return err
	}

	record, err := s.repo.FindRecord(id)
	if err != nil {
		return err
	}
	if !scope.CanAccessCompany(record.CompaniesID) {
		return ErrSalaryCompanyDenied
	}

	// The lock and the in-transaction usage check keep a cycle from closing between "this record was never used"
	// and the delete. Nothing is promoted afterwards: the record before it is the latest one by simply still being there.
	return s.repo.WithinTx(func(repo repository.SalariesStructureRepository) error {
		if err := repo.LockCompanyCycles(record.CompaniesID); err != nil {
			return err
		}

		usage, err := repo.ClosedCyclesUsing([]uint{record.ID})
		if err != nil {
			return err
		}
		if len(usage[record.ID]) > 0 {
			return ErrSalaryRecordInUse
		}

		if err := repo.DeleteRecord(&record); err != nil {
			return err
		}

		// The record id is left out: the row it pointed at is gone, and an entry that outlives its subject is
		// the point of keeping the trail separate from the timeline.
		amount, on := record.BaseSalaries, record.EffectiveDate
		return recordAudit(repo, scope, models.SalaryAudit{
			CompaniesID: record.CompaniesID,
			EmployeesID: record.EmployeesID,
			Action:      models.SalaryAuditDeleted,
			Reason:      reason,
			FromAmount:  &amount,
			FromDate:    &on,
		})
	})
}

package service

import (
	"errors"
	"testing"
	"time"

	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// fakeSalaryRepo is an in-memory SalariesStructureRepository that records the order of the calls a write path makes, and whether each one ran inside a transaction.
type fakeSalaryRepo struct {
	records      []models.SalariesStructure
	usage        map[uint][]string
	closedCycles int64
	findRecord   models.SalariesStructure

	txDepth     int
	calls       []string
	outsideTx   []string
	created     models.SalariesStructure
	switchedOff bool
	corrected   struct {
		record   models.SalariesStructure
		amount   float64
		switchOn bool
	}
	moved struct {
		record        models.SalariesStructure
		effectiveDate time.Time
		amount        float64
	}
	audits []models.SalaryAudit
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

func (f *fakeSalaryRepo) ClosedCyclesUsing([]uint) (map[uint][]string, error) {
	f.record("ClosedCyclesUsing")
	if f.usage == nil {
		return map[uint][]string{}, nil
	}
	return f.usage, nil
}

func (f *fakeSalaryRepo) CountClosedCyclesCovering(uint, time.Time) (int64, error) {
	f.record("CountClosedCyclesCovering")
	return f.closedCycles, nil
}

func (f *fakeSalaryRepo) SwitchOffAll(uint) error {
	f.record("SwitchOffAll")
	f.switchedOff = true
	return nil
}

func (f *fakeSalaryRepo) CreateRecord(next *models.SalariesStructure) error {
	f.record("CreateRecord")
	f.created = *next
	return nil
}

func (f *fakeSalaryRepo) UpdateAmount(record models.SalariesStructure, amount float64, switchOn bool) error {
	f.record("UpdateAmount")
	f.corrected.record, f.corrected.amount, f.corrected.switchOn = record, amount, switchOn
	return nil
}

func (f *fakeSalaryRepo) MoveRecord(record models.SalariesStructure, effectiveDate time.Time, amount float64) error {
	f.record("MoveRecord")
	f.moved.record, f.moved.effectiveDate, f.moved.amount = record, effectiveDate, amount
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

// Outside the write paths these tests exercise.
func (f *fakeSalaryRepo) ListEmployeesWithSalary(repository.AccessScope) ([]models.Employees, error) {
	return nil, nil
}

func (f *fakeSalaryRepo) ListRecordsForEmployee(uint) ([]models.SalariesStructure, error) {
	f.record("ListRecordsForEmployee")
	return f.records, nil
}

func (f *fakeSalaryRepo) AppendAudit(entry *models.SalaryAudit) error {
	f.record("AppendAudit")
	f.audits = append(f.audits, *entry)
	return nil
}

func (f *fakeSalaryRepo) ListAuditForEmployee(uint) ([]models.SalaryAudit, error) {
	f.record("ListAuditForEmployee")
	return f.audits, nil
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
		Reason:        "test reason",
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
	if repo.calls[len(repo.calls)-2] != "CreateRecord" || repo.calls[len(repo.calls)-1] != "AppendAudit" {
		t.Errorf("calls = %v, want the write and its audit entry last", repo.calls)
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
		Reason:        "test reason",
	})
	if !errors.Is(err, ErrSalaryDateInClosed) {
		t.Fatalf("Save error = %v, want %v", err, ErrSalaryDateInClosed)
	}
	for _, call := range repo.calls {
		if call == "CreateRecord" {
			t.Fatalf("calls = %v, want no write after the closed-cycle check failed", repo.calls)
		}
	}
}

// TestSalaryDeleteRunsInsideOneTransaction pins where the delete happens: the check that it is allowed and the
// delete itself share a transaction, so a record can not be erased on the strength of a check that has gone stale.
func TestSalaryDeleteRunsInsideOneTransaction(t *testing.T) {
	repo := &fakeSalaryRepo{
		findRecord: models.SalariesStructure{ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if err := NewSalariesStructureService(repo, employees).DeleteRecord(salaryScope(), 9, "test reason"); err != nil {
		t.Fatalf("DeleteRecord returned error: %v", err)
	}

	if len(repo.outsideTx) != 1 || repo.outsideTx[0] != "FindRecord" {
		t.Errorf("calls outside the transaction = %v, want only the FindRecord that resolves access", repo.outsideTx)
	}
	want := []string{"FindRecord", "LockCompanyCycles", "ClosedCyclesUsing", "DeleteRecord", "AppendAudit"}
	if len(repo.calls) != len(want) {
		t.Fatalf("calls = %v, want %v", repo.calls, want)
	}
	for i, call := range want {
		if repo.calls[i] != call {
			t.Fatalf("calls = %v, want %v", repo.calls, want)
		}
	}
}

// TestSalaryDeleteLeavesTheRestOfTheTimelineAlone covers what replaced the old promotion step: deleting the
// latest record does not touch any other record, because which one is in force is read from the dates afterwards.
func TestSalaryDeleteLeavesTheRestOfTheTimelineAlone(t *testing.T) {
	repo := &fakeSalaryRepo{
		findRecord: models.SalariesStructure{ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 4},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if err := NewSalariesStructureService(repo, employees).DeleteRecord(salaryScope(), 9, "test reason"); err != nil {
		t.Fatalf("DeleteRecord returned error: %v", err)
	}
	for _, call := range repo.calls {
		if call == "UpdateAmount" || call == "CreateRecord" || call == "SwitchOffAll" {
			t.Fatalf("calls = %v, want the delete to write nothing but the delete", repo.calls)
		}
	}
}

// TestSalarySaveRefusesStaleVersion covers the optimistic lock: a form seeded from an older version of the
// current record must be refused, not allowed to overwrite whatever landed in between.
func TestSalarySaveRefusesStaleVersion(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 7, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 3},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	_, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID:   5,
		CompaniesID:   1,
		BaseSalaries:  31000,
		EffectiveDate: "2026-02-01",
		Version:       2,
		Reason:        "test reason",
	})
	if !errors.Is(err, ErrSalaryVersionConflict) {
		t.Fatalf("Save error = %v, want %v", err, ErrSalaryVersionConflict)
	}
	for _, call := range repo.calls {
		if call == "CreateRecord" {
			t.Fatalf("calls = %v, want no write once the version check failed", repo.calls)
		}
	}
}

// TestSalarySaveAcceptsCurrentVersion is the other half: the version the caller read is the one on the record, so the save goes through.
func TestSalarySaveAcceptsCurrentVersion(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 7, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 3},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID:   5,
		CompaniesID:   1,
		BaseSalaries:  31000,
		EffectiveDate: "2026-02-01",
		Version:       3,
		Reason:        "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	// The audit entry is written last, inside the same transaction: a change that cannot be recorded is not made.
	if repo.calls[len(repo.calls)-2] != "CreateRecord" || repo.calls[len(repo.calls)-1] != "AppendAudit" {
		t.Fatalf("calls = %v, want the write followed by its audit entry", repo.calls)
	}
	if repo.created.Version != 4 {
		t.Fatalf("created version = %d, want 4: the replacement continues the employee's count", repo.created.Version)
	}
}

// TestSalarySaveRefusesSecondEditOfTheSameVersion is the race the lock exists for: two people seed a form from
// the same record, the first save supersedes it with a new row, and the second must be refused even though the
// row it now faces is brand new. A version that counted per row would restart at 1 here and let the second win.
func TestSalarySaveRefusesSecondEditOfTheSameVersion(t *testing.T) {
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 7, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 1},
		},
	}
	service := NewSalariesStructureService(repo, employees)

	// Both forms were seeded while the current record was at version 1.
	seeded := uint(1)

	if _, err := service.Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 35000, EffectiveDate: "2026-02-01", Version: seeded,
		Reason: "test reason",
	}); err != nil {
		t.Fatalf("first save returned error: %v", err)
	}

	// The first save replaced the record; what is current now is the row it created.
	repo.records = []models.SalariesStructure{
		{ID: 8, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: repo.created.Version},
	}
	repo.calls = nil

	_, err := service.Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 32000, EffectiveDate: "2026-02-01", Version: seeded,
		Reason: "test reason",
	})
	if !errors.Is(err, ErrSalaryVersionConflict) {
		t.Fatalf("second save error = %v, want %v", err, ErrSalaryVersionConflict)
	}
	for _, call := range repo.calls {
		if call == "CreateRecord" {
			t.Fatal("the second save wrote over the first")
		}
	}
}

// TestSalarySaveIgnoresErasedRecords covers the row an erase leaves behind. Records are read unscoped, so an
// erased one is still in the list; if it counted as active the save would try to archive a row that soft delete
// puts out of reach of every UPDATE, and report a conflict that is not one.
func TestSalarySaveIgnoresErasedRecords(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			// Erased by an earlier save, and left carrying is_active as the old code did.
			{ID: 19, EmployeesID: 5, CompaniesID: 1, IsActive: true, DeletedAt: 1787543463108934800, Version: 3},
			{ID: 20, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 3},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 52003, EffectiveDate: "2028-04-01", Version: 3, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if repo.created.Version != 4 {
		t.Fatalf("created version = %d, want 4: the erased record is not the one in force", repo.created.Version)
	}
	if repo.created.Version != 4 {
		t.Fatalf("created version = %d, want 4", repo.created.Version)
	}
}

// TestSalarySaveAfterADeleteLeftAnOlderRecordInForce covers what a delete leaves behind: the record before it is
// the latest one again, and its version is behind whatever the erased records reached. The form is seeded from
// that record, so the check compares against it — reading the highest version in the history instead would refuse
// every save from then on.
func TestSalarySaveAfterADeleteLeftAnOlderRecordInForce(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			// Back to being the latest record after a delete; its version never moved.
			{ID: 11, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 2},
			// Erased along the way, and further along the count than the record now in charge.
			{ID: 27, EmployeesID: 5, CompaniesID: 1, DeletedAt: 1787565590399885000, Version: 7},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	// The page shows record #11, so the form was seeded with 2.
	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 91000, EffectiveDate: "2029-01-01", Version: 2, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if repo.created.Version != 8 {
		t.Fatalf("created version = %d, want 8: one past the highest number this employee ever used", repo.created.Version)
	}
}

// TestSalarySaveOnTheSameEffectiveDateCorrectsInPlace covers a correction: the same date cannot mean two
// different figures, so the record that holds that date is fixed rather than superseded by a second one.
func TestSalarySaveOnTheSameEffectiveDateCorrectsInPlace(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 7, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 2, BaseSalaries: 30000,
				EffectiveDate: time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 31000, EffectiveDate: "2029-01-01", Version: 2, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if repo.corrected.record.ID != 7 || repo.corrected.record.Version != 2 || repo.corrected.amount != 31000 {
		t.Fatalf("corrected = %+v, want record 7 at version 2 set to 31000", repo.corrected)
	}
	for _, call := range repo.calls {
		if call == "CreateRecord" {
			t.Fatal("a correction must not add a second record for the same date")
		}
	}
}

// TestSalarySaveOnALaterDateKeepsThePreviousRecord is the rule a raise follows: the record it replaces stops being
// current and stays in the history. Erasing it would throw away the salary that was genuinely in force until then.
func TestSalarySaveOnALaterDateKeepsThePreviousRecord(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 7, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 2, BaseSalaries: 30000,
				EffectiveDate: time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 35000, EffectiveDate: "2029-06-01", Version: 2, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if repo.created.Version != 3 || repo.created.BaseSalaries != 35000 {
		t.Fatalf("created = %+v, want the new record at version 3", repo.created)
	}
	if repo.corrected.record.ID != 0 {
		t.Fatal("a later date must add a record, not correct the previous one")
	}
}

// TestSalarySaveMayGoBackOverADeletedRecord is what delete is for: once a record is out of the history, the date
// it occupied is free again. Counting deleted records in the chronology rule would leave the date blocked for ever
// and make deleting the record pointless.
func TestSalarySaveMayGoBackOverADeletedRecord(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 5, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 1, BaseSalaries: 38000,
				EffectiveDate: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)},
			// Deleted from the history, and dated after the record that is left.
			{ID: 23, EmployeesID: 5, CompaniesID: 1, Version: 2, BaseSalaries: 41000,
				DeletedAt:     1787565590399885000,
				EffectiveDate: time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 39000, EffectiveDate: "2027-01-01", Version: 1, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if repo.created.BaseSalaries != 39000 {
		t.Fatalf("created = %+v, want the record dated 2027 to have been written", repo.created)
	}
}

// TestSalarySaveFillsInAnEarlierDateWithoutTakingOver covers filling a gap in the history: the record lands where
// its date puts it and the salary in force today is left alone. A run prices each cycle from the record in force
// on that cycle's dates, so the new record changes what the earlier period pays and nothing else.
func TestSalarySaveFillsInAnEarlierDateWithoutTakingOver(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 5, EmployeesID: 5, CompaniesID: 1, Version: 1, BaseSalaries: 38000,
				EffectiveDate: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)},
			{ID: 23, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 2, BaseSalaries: 41000,
				EffectiveDate: time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 39000, EffectiveDate: "2027-01-01", Version: 2, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	// The record is switched on — it is the rate its own period is paid at. What it is not is the record in
	// force today, and that follows from the dates alone: nothing else was written.
	if !repo.created.IsActive {
		t.Fatal("a record added to the timeline is switched on")
	}
	for _, call := range repo.calls {
		if call == "UpdateAmount" || call == "SwitchOffAll" {
			t.Fatalf("calls = %v, want the records already on the timeline left untouched", repo.calls)
		}
	}
}

// TestSalaryCorrectionOfAnOlderRecordLeavesTodaysSalaryAlone is the same idea for a correction: fixing what an
// earlier period pays must not drag that record back into force over the one that superseded it.
func TestSalaryCorrectionOfAnOlderRecordLeavesTodaysSalaryAlone(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 5, EmployeesID: 5, CompaniesID: 1, Version: 1, BaseSalaries: 38000,
				EffectiveDate: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)},
			{ID: 23, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 2, BaseSalaries: 41000,
				EffectiveDate: time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 37000, EffectiveDate: "2026-01-01", Version: 2, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if repo.corrected.record.ID != 5 || repo.corrected.amount != 37000 {
		t.Fatalf("corrected = %+v, want record 5 set to 37000", repo.corrected)
	}
}

// TestSalarySaveOnTheDateOfASwitchedOffRecordRevivesIt is how a disabled salary is turned back on: saving on the
// date that record already holds corrects its figure and switches it back on, instead of colliding with it.
func TestSalarySaveOnTheDateOfASwitchedOffRecordRevivesIt(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			// Everything was switched off, so nothing is current.
			{ID: 5, EmployeesID: 5, CompaniesID: 1, Version: 3, BaseSalaries: 38000,
				EffectiveDate: time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 39000, EffectiveDate: "2029-01-01", Version: 0, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if repo.corrected.record.ID != 5 || repo.corrected.amount != 39000 {
		t.Fatalf("corrected = %+v, want the switched-off record 5 corrected to 39000", repo.corrected)
	}
	if repo.created.BaseSalaries != 0 {
		t.Fatalf("created = %+v, want no second record for the same date", repo.created)
	}
}

// TestSalarySaveRefusesToCorrectADatePaidAgainst protects a payslip that has already been issued: the record a
// closed cycle was calculated from is fixed, and the new figure belongs on a later effective date.
func TestSalarySaveRefusesToCorrectADatePaidAgainst(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 5, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 3, BaseSalaries: 38000,
				EffectiveDate: time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC)},
		},
		usage: map[uint][]string{5: {"March 2027"}},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	_, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 39000, EffectiveDate: "2029-01-01", Version: 3, Reason: "test reason",
	})
	if !errors.Is(err, ErrSalaryRecordPaid) {
		t.Fatalf("Save error = %v, want %v", err, ErrSalaryRecordPaid)
	}
	if repo.corrected.record.ID != 0 {
		t.Fatal("nothing may be corrected once a closed cycle was calculated from it")
	}
}

// TestSalarySaveWhenEverySalaryWasDeleted pins that the count never restarts. With nothing in force there is no
// version for a form to be stale against, so the save goes through — but the record it writes still takes a number
// past everything the employee has used, deleted records included, so no two records ever carry the same one.
func TestSalarySaveWhenEverySalaryWasDeleted(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 1, EmployeesID: 5, CompaniesID: 1, DeletedAt: 1787542871745053400, Version: 1},
			{ID: 2, EmployeesID: 5, CompaniesID: 1, DeletedAt: 1787565261329692800, Version: 2},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	// The page shows no salary at all, so the form sends no version.
	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 40000, EffectiveDate: "2029-01-01", Version: 0, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if repo.created.Version != 3 {
		t.Fatalf("created version = %d, want 3: past the erased records, whose numbers are not reused", repo.created.Version)
	}
}

// TestSalarySaveNeverReusesAVersion covers the number handed to a record added behind the one in force. Counting
// from the record in force would hand the same number out twice, since adding to the past leaves it untouched.
func TestSalarySaveNeverReusesAVersion(t *testing.T) {
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 10, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 3,
				EffectiveDate: time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	service := NewSalariesStructureService(repo, employees)

	// Filling in an earlier period does not change what is in force, so the form is still seeded with version 3.
	if _, err := service.Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 39000, EffectiveDate: "2027-01-01", Version: 3, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	backfilled := repo.created.Version
	repo.records = append(repo.records, models.SalariesStructure{
		ID: 11, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: backfilled,
		EffectiveDate: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)})

	if _, err := service.Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 45000, EffectiveDate: "2031-01-01", Version: 3, Reason: "test reason",
	}); err != nil {
		t.Fatalf("second Save returned error: %v", err)
	}
	if repo.created.Version == backfilled {
		t.Fatalf("two records were written with version %d", backfilled)
	}
}

// TestSalaryCorrectionOfAnOlderRecordDoesNotSwitchItOn covers a disabled salary being corrected on an old date:
// the figure that period pays is fixed, but the record does not come back into use over a later one.
func TestSalaryCorrectionOfAnOlderRecordDoesNotSwitchItOn(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			// Both switched off — the salary was disabled.
			{ID: 5, EmployeesID: 5, CompaniesID: 1, Version: 1, BaseSalaries: 38000,
				EffectiveDate: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)},
			{ID: 23, EmployeesID: 5, CompaniesID: 1, Version: 2, BaseSalaries: 41000,
				EffectiveDate: time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 37000, EffectiveDate: "2026-01-01", Version: 0, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if repo.corrected.record.ID != 5 || repo.corrected.amount != 37000 {
		t.Fatalf("corrected = %+v, want record 5 set to 37000", repo.corrected)
	}
	if repo.corrected.switchOn {
		t.Fatal("an older record must not be switched on while a later one sits switched off")
	}
}

// TestSalaryCorrectionOfTheLatestRecordSwitchesItOn is the other half: correcting the record with the latest date
// is how a disabled salary is put back into use.
func TestSalaryCorrectionOfTheLatestRecordSwitchesItOn(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 5, EmployeesID: 5, CompaniesID: 1, Version: 1, BaseSalaries: 38000,
				EffectiveDate: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)},
			{ID: 23, EmployeesID: 5, CompaniesID: 1, Version: 2, BaseSalaries: 41000,
				EffectiveDate: time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 42000, EffectiveDate: "2030-01-01", Version: 0, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if repo.corrected.record.ID != 23 || !repo.corrected.switchOn {
		t.Fatalf("corrected = %+v, want record 23 switched back on", repo.corrected)
	}
}

// timeline builds a fake holding one employee's records, for the move tests below.
func timeline(records ...models.SalariesStructure) *fakeSalaryRepo {
	return &fakeSalaryRepo{records: records, findRecord: records[0]}
}

func salaryOn(id uint, version uint, amount float64, year int, month time.Month) models.SalariesStructure {
	return models.SalariesStructure{
		ID: id, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: version, BaseSalaries: amount,
		EffectiveDate: time.Date(year, month, 1, 0, 0, 0, 0, time.UTC),
	}
}

// TestSalaryMoveShiftsARecordAlongTheTimeline is the move itself: the record keeps its identity and its place in
// the history, and only the date it takes effect from changes.
func TestSalaryMoveShiftsARecordAlongTheTimeline(t *testing.T) {
	repo := timeline(
		salaryOn(23, 2, 35000, 2030, time.July),
		salaryOn(5, 1, 30000, 2029, time.January),
	)
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if err := NewSalariesStructureService(repo, employees).UpdateRecord(salaryScope(), 23, SalaryRecordEdit{
		EffectiveDate: "2030-06-01", BaseSalaries: 35000, Version: 2, Reason: "test reason",
	}); err != nil {
		t.Fatalf("UpdateRecord returned error: %v", err)
	}
	if repo.moved.record.ID != 23 || repo.moved.effectiveDate.Month() != time.June {
		t.Fatalf("moved = %+v, want record 23 moved to June", repo.moved)
	}
	for _, call := range repo.calls {
		if call == "CreateRecord" || call == "UpdateAmount" {
			t.Fatalf("calls = %v, want a move to write nothing else", repo.calls)
		}
	}
}

// TestSalaryMoveRefusesADateAnotherRecordHolds keeps one date to one record, and says so rather than letting the
// unique index fail with something the page cannot explain.
func TestSalaryMoveRefusesADateAnotherRecordHolds(t *testing.T) {
	repo := timeline(
		salaryOn(23, 2, 35000, 2030, time.July),
		salaryOn(5, 1, 30000, 2029, time.January),
	)
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	err := NewSalariesStructureService(repo, employees).UpdateRecord(salaryScope(), 23, SalaryRecordEdit{
		EffectiveDate: "2029-01-01", BaseSalaries: 35000, Version: 2, Reason: "test reason",
	})
	if !errors.Is(err, ErrSalaryDateTaken) {
		t.Fatalf("UpdateRecord error = %v, want %v", err, ErrSalaryDateTaken)
	}
	if repo.moved.record.ID != 0 {
		t.Fatal("nothing may move onto a date another record holds")
	}
}

// TestSalaryMoveRefusesARecordAlreadyPaidAgainst protects a payslip that has been issued: the record it was
// calculated from stays where it is and keeps its figure.
func TestSalaryMoveRefusesARecordAlreadyPaidAgainst(t *testing.T) {
	repo := timeline(salaryOn(23, 2, 35000, 2030, time.July))
	repo.usage = map[uint][]string{23: {"March 2027"}}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	err := NewSalariesStructureService(repo, employees).UpdateRecord(salaryScope(), 23, SalaryRecordEdit{
		EffectiveDate: "2030-06-01", BaseSalaries: 35000, Version: 2, Reason: "test reason",
	})
	if !errors.Is(err, ErrSalaryRecordPaid) {
		t.Fatalf("UpdateRecord error = %v, want %v", err, ErrSalaryRecordPaid)
	}
	if repo.moved.record.ID != 0 {
		t.Fatal("a record a closed cycle was calculated from must not move")
	}
}

// TestSalaryMoveRefusesLandingInAClosedPeriod covers the other side of the same rule: the date it moves to must
// not fall inside a cycle that has already been paid out.
func TestSalaryMoveRefusesLandingInAClosedPeriod(t *testing.T) {
	repo := timeline(salaryOn(23, 2, 35000, 2030, time.July))
	repo.closedCycles = 1
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	err := NewSalariesStructureService(repo, employees).UpdateRecord(salaryScope(), 23, SalaryRecordEdit{
		EffectiveDate: "2026-03-01", BaseSalaries: 35000, Version: 2, Reason: "test reason",
	})
	if !errors.Is(err, ErrSalaryDateInClosed) {
		t.Fatalf("UpdateRecord error = %v, want %v", err, ErrSalaryDateInClosed)
	}
	if repo.moved.record.ID != 0 {
		t.Fatal("nothing may move into a period already paid out")
	}
}

// TestSalaryMoveRefusesAStaleEdit is the optimistic lock on the row being edited: the version in the request is
// the one the screen was drawn from, and the edit is refused once the record has moved on from it.
func TestSalaryMoveRefusesAStaleEdit(t *testing.T) {
	repo := timeline(salaryOn(23, 4, 35000, 2030, time.July))
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	err := NewSalariesStructureService(repo, employees).UpdateRecord(salaryScope(), 23, SalaryRecordEdit{
		EffectiveDate: "2030-06-01", BaseSalaries: 35000, Version: 2, Reason: "test reason",
	})
	if !errors.Is(err, ErrSalaryVersionConflict) {
		t.Fatalf("UpdateRecord error = %v, want %v", err, ErrSalaryVersionConflict)
	}
	if repo.moved.record.ID != 0 {
		t.Fatal("a stale edit must not move anything")
	}
}

// TestSalaryMoveRunsInsideOneTransaction pins that the checks and the write share a transaction, with the cycle
// lock taken first: every rule a move enforces is a question about cycles that could otherwise close underneath it.
func TestSalaryMoveRunsInsideOneTransaction(t *testing.T) {
	repo := timeline(salaryOn(23, 2, 35000, 2030, time.July))
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if err := NewSalariesStructureService(repo, employees).UpdateRecord(salaryScope(), 23, SalaryRecordEdit{
		EffectiveDate: "2030-06-01", BaseSalaries: 36000, Version: 2, Reason: "test reason",
	}); err != nil {
		t.Fatalf("UpdateRecord returned error: %v", err)
	}
	if len(repo.outsideTx) != 1 || repo.outsideTx[0] != "FindRecord" {
		t.Errorf("calls outside the transaction = %v, want only the FindRecord that resolves access", repo.outsideTx)
	}
	if repo.calls[1] != "LockCompanyCycles" {
		t.Fatalf("calls = %v, want the cycle lock taken first inside the transaction", repo.calls)
	}
	if repo.calls[len(repo.calls)-2] != "MoveRecord" || repo.calls[len(repo.calls)-1] != "AppendAudit" {
		t.Fatalf("calls = %v, want the move followed by its audit entry", repo.calls)
	}
}

// TestSalaryDeleteRefusesRecordUsedByClosedCycle keeps the audit rule inside the transaction.
func TestSalaryDeleteRefusesRecordUsedByClosedCycle(t *testing.T) {
	repo := &fakeSalaryRepo{
		findRecord: models.SalariesStructure{ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true},
		usage:      map[uint][]string{9: {"March 2027"}},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	err := NewSalariesStructureService(repo, employees).DeleteRecord(salaryScope(), 9, "test reason")
	if !errors.Is(err, ErrSalaryRecordInUse) {
		t.Fatalf("DeleteRecord error = %v, want %v", err, ErrSalaryRecordInUse)
	}
	for _, call := range repo.calls {
		if call == "DeleteRecord" {
			t.Fatal("DeleteRecord was called for a record a closed cycle used")
		}
	}
}

// A row is read-only because a closed cycle was paid from it, and History carries the names of those cycles
// so the screen can say which ones rather than only that the row is locked. CanDelete is derived from that
// same list, so the two can never disagree.
func TestSalaryHistoryNamesTheClosedCyclesThatLockARow(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 5, EmployeesID: 5, CompaniesID: 1, IsActive: true, BaseSalaries: 38000,
				EffectiveDate: time.Date(2027, time.January, 1, 0, 0, 0, 0, timeutil.BangkokLocation)},
			{ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true, BaseSalaries: 41000,
				EffectiveDate: time.Date(2027, time.July, 1, 0, 0, 0, 0, timeutil.BangkokLocation)},
		},
		usage: map[uint][]string{5: {"January 2027", "February 2027", "March 2027"}},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	entries, err := NewSalariesStructureService(repo, employees).History(salaryScope(), 5)
	if err != nil {
		t.Fatalf("History returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	byID := map[uint]SalaryHistoryEntry{}
	for _, entry := range entries {
		byID[entry.Record.ID] = entry
	}

	locked := byID[5]
	want := []string{"January 2027", "February 2027", "March 2027"}
	if len(locked.LockedBy) != len(want) {
		t.Fatalf("LockedBy = %v, want %v", locked.LockedBy, want)
	}
	for i, name := range want {
		if locked.LockedBy[i] != name {
			t.Fatalf("LockedBy[%d] = %q, want %q — the order the cycles ran in", i, locked.LockedBy[i], name)
		}
	}
	if locked.CanDelete {
		t.Fatal("a record three closed cycles were paid from must not be deletable")
	}

	free := byID[9]
	if len(free.LockedBy) != 0 {
		t.Fatalf("LockedBy = %v for a record no closed cycle used, want empty", free.LockedBy)
	}
	if !free.CanDelete {
		t.Fatal("a record no closed cycle used stays deletable")
	}
}

// Omitting the version used to skip the optimistic lock on a move, which handed the last writer the record.
// A zero is now compared like any other value, so it only passes when the stored version is zero too.
func TestSalaryMoveRefusesAnEditThatOmitsTheVersion(t *testing.T) {
	record := models.SalariesStructure{ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 3,
		EffectiveDate: time.Date(2027, time.March, 1, 0, 0, 0, 0, timeutil.BangkokLocation)}
	repo := timeline(record)
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	err := NewSalariesStructureService(repo, employees).UpdateRecord(salaryScope(), 9, SalaryRecordEdit{
		EffectiveDate: "2027-04-01", BaseSalaries: 50000, Version: 0, Reason: "test reason",
	})
	if !errors.Is(err, ErrSalaryVersionConflict) {
		t.Fatalf("UpdateRecord error = %v, want %v", err, ErrSalaryVersionConflict)
	}
	if repo.moved.record.ID != 0 {
		t.Fatal("nothing may move on an edit that carried no version")
	}
}

// A record whose stored version is zero — a row that predates the column — is still editable by a caller
// that read that zero and sent it back.
func TestSalaryMoveAcceptsAZeroVersionWhenTheRecordCarriesOne(t *testing.T) {
	record := models.SalariesStructure{ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 0,
		EffectiveDate: time.Date(2027, time.March, 1, 0, 0, 0, 0, timeutil.BangkokLocation)}
	repo := timeline(record)
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if err := NewSalariesStructureService(repo, employees).UpdateRecord(salaryScope(), 9, SalaryRecordEdit{
		EffectiveDate: "2027-04-01", BaseSalaries: 50000, Version: 0, Reason: "test reason",
	}); err != nil {
		t.Fatalf("UpdateRecord returned %v, want nil", err)
	}
	if repo.moved.record.ID != 9 {
		t.Fatalf("moved record = %d, want 9", repo.moved.record.ID)
	}
}

// The Save path compares the version too, and the zero it compares against is not a special case: an
// employee with nothing in force genuinely has version zero, which is what the form is seeded with. These
// three pin that removing the old `input.Version > 0` short-circuit closed the hole without shutting
// anything legitimate out.
func TestSalarySaveAcceptsAZeroVersionForAnEmployeeWithNoSalary(t *testing.T) {
	repo := &fakeSalaryRepo{}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 30000, EffectiveDate: "2029-02-01", Version: 0, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned %v, want nil — a first salary has no earlier version to send", err)
	}
	if repo.created.BaseSalaries != 30000 {
		t.Fatalf("created %v, want the record to be written", repo.created)
	}
}

// Nothing is in force while every record is switched off, so the version the form was seeded with is zero
// there too, and saving must still be allowed.
func TestSalarySaveAcceptsAZeroVersionWhenEverythingIsSwitchedOff(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 5, EmployeesID: 5, CompaniesID: 1, IsActive: false, Version: 3, BaseSalaries: 38000,
				EffectiveDate: time.Date(2027, time.January, 1, 0, 0, 0, 0, timeutil.BangkokLocation)},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 41000, EffectiveDate: "2029-02-01", Version: 0, Reason: "test reason",
	}); err != nil {
		t.Fatalf("Save returned %v, want nil — nothing is in force, so the seeded version is zero", err)
	}
}

// But an employee who does have a salary in force has a version, and a save that omits it is a stale screen
// rather than a create.
func TestSalarySaveRefusesAZeroVersionWhenARecordIsInForce(t *testing.T) {
	repo := &fakeSalaryRepo{
		records: []models.SalariesStructure{
			{ID: 5, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 3, BaseSalaries: 38000,
				EffectiveDate: time.Date(2027, time.January, 1, 0, 0, 0, 0, timeutil.BangkokLocation)},
		},
	}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	_, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 41000, EffectiveDate: "2029-02-01", Version: 0, Reason: "test reason",
	})
	if !errors.Is(err, ErrSalaryVersionConflict) {
		t.Fatalf("Save error = %v, want %v", err, ErrSalaryVersionConflict)
	}
	if repo.created.BaseSalaries != 0 {
		t.Fatal("nothing may be written on a save that carried no version")
	}
}

// Every salary write records who made it and why, in the same transaction as the change, so a change that
// cannot be recorded is not made. These pin the shape of the entry each path leaves behind.
func TestSalaryWritesRecordWhoChangedItAndWhy(t *testing.T) {
	actor := repository.NewAccessScope(
		models.Employees{ID: 7, CompaniesID: 1, FirstName: "Grace", LastName: "Hall"},
		models.ScopeCompanies, []uint{1})
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	t.Run("creating a record", func(t *testing.T) {
		repo := &fakeSalaryRepo{}
		if _, err := NewSalariesStructureService(repo, employees).Save(actor, SalaryInput{
			EmployeesID: 5, CompaniesID: 1, BaseSalaries: 30000, EffectiveDate: "2029-02-01",
			Reason: "annual review",
		}); err != nil {
			t.Fatalf("Save returned %v", err)
		}
		if len(repo.audits) != 1 {
			t.Fatalf("wrote %d audit entries, want 1", len(repo.audits))
		}
		entry := repo.audits[0]
		if entry.Action != models.SalaryAuditCreated {
			t.Errorf("action = %q, want %q", entry.Action, models.SalaryAuditCreated)
		}
		if entry.Reason != "annual review" {
			t.Errorf("reason = %q, want the reason the caller gave", entry.Reason)
		}
		if entry.ActorID != 7 || entry.ActorName != "Grace Hall" {
			t.Errorf("actor = %d/%q, want 7/\"Grace Hall\" — the name is snapshotted, not joined", entry.ActorID, entry.ActorName)
		}
		if entry.FromAmount != nil {
			t.Error("a create has nothing to come from")
		}
		if entry.ToAmount == nil || *entry.ToAmount != 30000 {
			t.Errorf("to amount = %v, want 30000", entry.ToAmount)
		}
	})

	t.Run("moving a record", func(t *testing.T) {
		repo := timeline(models.SalariesStructure{
			ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true, Version: 2, BaseSalaries: 38000,
			EffectiveDate: time.Date(2029, time.March, 1, 0, 0, 0, 0, timeutil.BangkokLocation)})
		if err := NewSalariesStructureService(repo, employees).UpdateRecord(actor, 9, SalaryRecordEdit{
			EffectiveDate: "2029-04-01", BaseSalaries: 41000, Version: 2, Reason: "start date corrected",
		}); err != nil {
			t.Fatalf("UpdateRecord returned %v", err)
		}
		entry := repo.audits[0]
		if entry.Action != models.SalaryAuditMoved {
			t.Errorf("action = %q, want %q", entry.Action, models.SalaryAuditMoved)
		}
		if entry.FromAmount == nil || *entry.FromAmount != 38000 || entry.ToAmount == nil || *entry.ToAmount != 41000 {
			t.Errorf("amounts = %v -> %v, want 38000 -> 41000", entry.FromAmount, entry.ToAmount)
		}
		if entry.FromDate == nil || entry.ToDate == nil {
			t.Fatal("a move records both dates")
		}
		if timeutil.DateKey(*entry.FromDate) != "2029-03-01" || timeutil.DateKey(*entry.ToDate) != "2029-04-01" {
			t.Errorf("dates = %s -> %s, want 2029-03-01 -> 2029-04-01",
				timeutil.DateKey(*entry.FromDate), timeutil.DateKey(*entry.ToDate))
		}
	})

	t.Run("deleting a record", func(t *testing.T) {
		repo := &fakeSalaryRepo{findRecord: models.SalariesStructure{
			ID: 9, EmployeesID: 5, CompaniesID: 1, IsActive: true, BaseSalaries: 38000,
			EffectiveDate: time.Date(2029, time.March, 1, 0, 0, 0, 0, timeutil.BangkokLocation)}}
		if err := NewSalariesStructureService(repo, employees).DeleteRecord(actor, 9, "entered twice"); err != nil {
			t.Fatalf("DeleteRecord returned %v", err)
		}
		entry := repo.audits[0]
		if entry.Action != models.SalaryAuditDeleted {
			t.Errorf("action = %q, want %q", entry.Action, models.SalaryAuditDeleted)
		}
		// The record is gone, so the entry cannot point at it and must still say what was removed.
		if entry.SalariesStructureID != nil {
			t.Error("a delete leaves no record to point at")
		}
		if entry.FromAmount == nil || *entry.FromAmount != 38000 {
			t.Errorf("from amount = %v, want the amount that was removed", entry.FromAmount)
		}
	})
}

// A write with nothing said about why is refused before anything is touched.
func TestSalaryWritesRefuseAnEmptyReason(t *testing.T) {
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	for _, reason := range []string{"", "   ", "\t\n"} {
		repo := &fakeSalaryRepo{}
		if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
			EmployeesID: 5, CompaniesID: 1, BaseSalaries: 30000, EffectiveDate: "2029-02-01", Reason: reason,
		}); !errors.Is(err, ErrSalaryReasonRequired) {
			t.Fatalf("Save(%q) error = %v, want %v", reason, err, ErrSalaryReasonRequired)
		}
		if len(repo.calls) != 0 {
			t.Fatalf("calls = %v, want nothing touched", repo.calls)
		}
	}

	repo := &fakeSalaryRepo{}
	if err := NewSalariesStructureService(repo, employees).DeleteRecord(salaryScope(), 9, " "); !errors.Is(err, ErrSalaryReasonRequired) {
		t.Fatalf("DeleteRecord error = %v, want %v", err, ErrSalaryReasonRequired)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("calls = %v, want nothing touched", repo.calls)
	}
}

// The reason is stored trimmed, so a trail is not padded with whatever the field happened to contain.
func TestSalaryAuditTrimsTheReason(t *testing.T) {
	repo := &fakeSalaryRepo{}
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1}}

	if _, err := NewSalariesStructureService(repo, employees).Save(salaryScope(), SalaryInput{
		EmployeesID: 5, CompaniesID: 1, BaseSalaries: 30000, EffectiveDate: "2029-02-01",
		Reason: "   promotion to senior   ",
	}); err != nil {
		t.Fatalf("Save returned %v", err)
	}
	if repo.audits[0].Reason != "promotion to senior" {
		t.Errorf("reason = %q, want it trimmed", repo.audits[0].Reason)
	}
}

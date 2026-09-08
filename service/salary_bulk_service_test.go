package service

import (
	"errors"
	"testing"
	"time"

	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// bulkRepo answers the reads a batch makes, per employee, and counts what it wrote. It embeds the
// single-employee fake so only the methods a batch actually uses need answering here.
type bulkRepo struct {
	*fakeSalaryRepo
	perEmployee  map[uint][]models.SalariesStructure
	closedFor    map[uint]int64
	paidRecords  map[uint][]string
	lockedOrder  []uint
	writtenCount int
}

func newBulkRepo() *bulkRepo {
	return &bulkRepo{
		fakeSalaryRepo: &fakeSalaryRepo{},
		perEmployee:    map[uint][]models.SalariesStructure{},
		closedFor:      map[uint]int64{},
		paidRecords:    map[uint][]string{},
	}
}

func (b *bulkRepo) WithinTx(fn func(repository.SalariesStructureRepository) error) error {
	return fn(b)
}

func (b *bulkRepo) LockCompanyCycles(companyID uint) error {
	b.lockedOrder = append(b.lockedOrder, companyID)
	return nil
}

func (b *bulkRepo) ListAllRecordsUnscoped(employeesID uint, _ uint) ([]models.SalariesStructure, error) {
	return b.perEmployee[employeesID], nil
}

func (b *bulkRepo) CountClosedCyclesCovering(companyID uint, _ time.Time) (int64, error) {
	return b.closedFor[companyID], nil
}

func (b *bulkRepo) ClosedCyclesUsing(ids []uint) (map[uint][]string, error) {
	out := map[uint][]string{}
	for _, id := range ids {
		if names := b.paidRecords[id]; len(names) > 0 {
			out[id] = names
		}
	}
	return out, nil
}

func (b *bulkRepo) CreateRecord(next *models.SalariesStructure) error {
	b.writtenCount++
	next.ID = uint(900 + b.writtenCount)
	return nil
}

func (b *bulkRepo) UpdateAmount(models.SalariesStructure, float64, bool) error {
	b.writtenCount++
	return nil
}

func (b *bulkRepo) AppendAuditLog(*models.AuditLog) error { return nil }

// bulkEmployees answers the authorization lookup for several people.
type bulkEmployees struct {
	byID map[uint]models.Employees
}

func (b *bulkEmployees) FindBriefScoped(_ repository.AccessScope, id uint) (models.Employees, error) {
	employee, ok := b.byID[id]
	if !ok {
		return models.Employees{}, errors.New("not found")
	}
	return employee, nil
}

func (b *bulkEmployees) ListScoped(repository.AccessScope) ([]models.Employees, error) {
	return nil, nil
}
func (b *bulkEmployees) FindScoped(repository.AccessScope, uint) (models.Employees, error) {
	return models.Employees{}, nil
}
func (b *bulkEmployees) FindRole(uint) (models.Roles, error)                  { return models.Roles{}, nil }
func (b *bulkEmployees) Create(*models.Employees) error                       { return nil }
func (b *bulkEmployees) Delete(*models.Employees) error                       { return nil }
func (b *bulkEmployees) FindDetail(uint) (models.Employees, error)            { return models.Employees{}, nil }
func (b *bulkEmployees) UpdateFields(*models.Employees, map[string]any) error { return nil }

// bulkFixture is three employees of company 1: two already paid, and one who has no salary at all.
func bulkFixture() (*bulkRepo, *bulkEmployees) {
	repo := newBulkRepo()
	on := func(id uint, amount float64) []models.SalariesStructure {
		return []models.SalariesStructure{{
			ID: id * 10, EmployeesID: id, CompaniesID: 1, IsActive: true, Version: 1, BaseSalaries: amount,
			EffectiveDate: time.Date(2027, time.January, 1, 0, 0, 0, 0, timeutil.BangkokLocation),
		}}
	}
	repo.perEmployee[5] = on(5, 40000)
	repo.perEmployee[6] = on(6, 50000)
	repo.perEmployee[7] = nil

	return repo, &bulkEmployees{byID: map[uint]models.Employees{
		5: {ID: 5, CompaniesID: 1, EmployeesCode: "E-05", FirstName: "Ann", LastName: "Lee"},
		6: {ID: 6, CompaniesID: 1, EmployeesCode: "E-06", FirstName: "Ben", LastName: "Ko"},
		7: {ID: 7, CompaniesID: 1, EmployeesCode: "E-07", FirstName: "Cat", LastName: "Ng"},
	}}
}

func bulkTargets(ids ...uint) []BulkRaiseTarget {
	targets := make([]BulkRaiseTarget, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, BulkRaiseTarget{EmployeesID: id})
	}
	return targets
}

// The preview works out each new figure and rounds it to the two decimals the column stores, so what the
// operator approves is what gets written.
func TestBulkRaisePreviewWorksOutEachNewFigure(t *testing.T) {
	repo, employees := bulkFixture()
	repo.perEmployee[5][0].BaseSalaries = 45250.50

	preview, err := NewSalariesStructureService(repo, employees).BulkRaisePlan(salaryScope(), BulkRaiseInput{
		Mode: BulkRaisePercent, Value: 5, EffectiveDate: "2029-01-01", Reason: "annual review",
		Targets: bulkTargets(5, 6),
	})
	if err != nil {
		t.Fatalf("BulkRaisePlan returned %v", err)
	}
	if preview.BlockedCount != 0 {
		t.Fatalf("blocked %d, want none: %+v", preview.BlockedCount, preview.Rows)
	}

	got := map[string]float64{}
	for _, row := range preview.Rows {
		got[row.EmployeesCode] = row.NewSalary
	}
	// 45,250.50 plus 5% is 47,513.025, which the column cannot hold.
	if got["E-05"] != 47513.03 {
		t.Errorf("E-05 new salary = %v, want 47513.03 rounded to the stored precision", got["E-05"])
	}
	if got["E-06"] != 52500 {
		t.Errorf("E-06 new salary = %v, want 52500", got["E-06"])
	}
	if repo.writtenCount != 0 {
		t.Errorf("preview wrote %d records, want none", repo.writtenCount)
	}
}

// Someone with no salary has nothing for a percentage to work from, and says so rather than being raised to zero.
func TestBulkRaiseBlocksAnEmployeeWithNoSalaryToRaiseFrom(t *testing.T) {
	repo, employees := bulkFixture()

	preview, err := NewSalariesStructureService(repo, employees).BulkRaisePlan(salaryScope(), BulkRaiseInput{
		Mode: BulkRaisePercent, Value: 5, EffectiveDate: "2029-01-01", Reason: "annual review",
		Targets: bulkTargets(5, 7),
	})
	if err != nil {
		t.Fatalf("BulkRaisePlan returned %v", err)
	}
	if preview.BlockedCount != 1 {
		t.Fatalf("blocked %d, want 1: %+v", preview.BlockedCount, preview.Rows)
	}
	// Blocked rows sort first, because they are why the batch will not go through.
	if preview.Rows[0].EmployeesCode != "E-07" || preview.Rows[0].Blocker == "" {
		t.Fatalf("first row = %+v, want the blocked employee with a reason", preview.Rows[0])
	}
}

// The same person is fine when a figure is typed for them, because manual mode needs no base to work from.
func TestBulkRaiseAcceptsAnEmployeeWithNoSalaryWhenTheFigureIsTyped(t *testing.T) {
	repo, employees := bulkFixture()

	preview, err := NewSalariesStructureService(repo, employees).BulkRaisePlan(salaryScope(), BulkRaiseInput{
		Mode: BulkRaiseManual, EffectiveDate: "2029-01-01", Reason: "first salary",
		Targets: []BulkRaiseTarget{{EmployeesID: 7, NewSalary: 28000}},
	})
	if err != nil {
		t.Fatalf("BulkRaisePlan returned %v", err)
	}
	if preview.BlockedCount != 0 {
		t.Fatalf("blocked %d, want none: %+v", preview.BlockedCount, preview.Rows)
	}
	if preview.Rows[0].NewSalary != 28000 {
		t.Errorf("new salary = %v, want the figure that was typed", preview.Rows[0].NewSalary)
	}
}

// A date inside a closed cycle blocks everyone in that company, and the batch refuses whole rather than
// raising the people who happen to be unaffected.
func TestBulkRaiseRefusesTheWholeBatchWhenAnyoneIsBlocked(t *testing.T) {
	repo, employees := bulkFixture()
	repo.closedFor[1] = 1

	preview, err := NewSalariesStructureService(repo, employees).BulkRaiseApply(salaryScope(), BulkRaiseInput{
		Mode: BulkRaiseAmount, Value: 2000, EffectiveDate: "2027-03-15", Reason: "annual review",
		Targets: bulkTargets(5, 6),
	})
	if !errors.Is(err, ErrBulkBlocked) {
		t.Fatalf("BulkRaiseApply error = %v, want %v", err, ErrBulkBlocked)
	}
	if repo.writtenCount != 0 {
		t.Fatalf("wrote %d records on a refused batch, want none", repo.writtenCount)
	}
	// The refusal still carries the preview, so the screen can name who stopped it.
	if preview.BlockedCount != 2 {
		t.Fatalf("preview blocked %d, want both employees named", preview.BlockedCount)
	}
}

// One person already paid from on that date stops the batch, even though everyone else is fine — this is the
// case the all-or-nothing rule exists for.
func TestBulkRaiseRefusesWhenOnePersonWasAlreadyPaidOnThatDate(t *testing.T) {
	repo, employees := bulkFixture()
	repo.paidRecords[60] = []string{"January 2027"}

	preview, err := NewSalariesStructureService(repo, employees).BulkRaiseApply(salaryScope(), BulkRaiseInput{
		Mode: BulkRaiseAmount, Value: 2000, EffectiveDate: "2027-01-01", Reason: "annual review",
		Targets: bulkTargets(5, 6),
	})
	if !errors.Is(err, ErrBulkBlocked) {
		t.Fatalf("BulkRaiseApply error = %v, want %v", err, ErrBulkBlocked)
	}
	if repo.writtenCount != 0 {
		t.Fatalf("wrote %d records, want none — the batch is all or nothing", repo.writtenCount)
	}
	if preview.BlockedCount != 1 || preview.Rows[0].EmployeesCode != "E-06" {
		t.Fatalf("preview = %+v, want only E-06 blocked", preview.Rows)
	}
}

// A batch that passes writes one record per person, inside one transaction that locks the company first.
func TestBulkRaiseAppliesEveryoneInOneTransaction(t *testing.T) {
	repo, employees := bulkFixture()

	if _, err := NewSalariesStructureService(repo, employees).BulkRaiseApply(salaryScope(), BulkRaiseInput{
		Mode: BulkRaiseAmount, Value: 2000, EffectiveDate: "2029-01-01", Reason: "annual review",
		Targets: bulkTargets(5, 6),
	}); err != nil {
		t.Fatalf("BulkRaiseApply returned %v", err)
	}
	if repo.writtenCount != 2 {
		t.Fatalf("wrote %d records, want 2", repo.writtenCount)
	}
	if len(repo.lockedOrder) != 1 || repo.lockedOrder[0] != 1 {
		t.Fatalf("locked %v, want the one company locked once", repo.lockedOrder)
	}
}

// The batch is refused before any employee is read when the request itself makes no sense.
func TestBulkRaiseRejectsAnUnusableRequest(t *testing.T) {
	cases := []struct {
		name  string
		input BulkRaiseInput
		want  error
	}{
		{"no one selected", BulkRaiseInput{Mode: BulkRaisePercent, Value: 5, Reason: "x"}, ErrBulkNoEmployees},
		{"unknown mode", BulkRaiseInput{Mode: "sideways", Value: 5, Reason: "x", Targets: bulkTargets(5)}, ErrBulkModeInvalid},
		{"a raise of nothing", BulkRaiseInput{Mode: BulkRaisePercent, Value: 0, Reason: "x", Targets: bulkTargets(5)}, ErrBulkValueInvalid},
		{"no reason", BulkRaiseInput{Mode: BulkRaisePercent, Value: 5, Reason: "  ", Targets: bulkTargets(5)}, ErrSalaryReasonRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, employees := bulkFixture()
			if _, err := NewSalariesStructureService(repo, employees).BulkRaiseApply(salaryScope(), tc.input); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if repo.writtenCount != 0 {
				t.Fatalf("wrote %d records, want none", repo.writtenCount)
			}
		})
	}
}

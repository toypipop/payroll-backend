package service

import (
	"math"
	"sort"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// Bulk raise rule failures.
var (
	ErrBulkNoEmployees  = apperr.Validation("Select at least one employee")
	ErrBulkModeInvalid  = apperr.Validation("Choose how the raise is worked out: percent, amount, or a figure per person")
	ErrBulkValueInvalid = apperr.Validation("The raise must be greater than zero")
	// The batch is refused whole, so the message points at the preview rather than naming one person.
	ErrBulkBlocked = apperr.Conflict("Some of the selected employees cannot be raised on this date. Review the preview and try again.")
)

// BulkRaiseMode is how a new figure is worked out from the current one.
type BulkRaiseMode string

const (
	// BulkRaisePercent adds a percentage of each employee's current salary.
	BulkRaisePercent BulkRaiseMode = "percent"
	// BulkRaiseAmount adds the same figure to everyone.
	BulkRaiseAmount BulkRaiseMode = "amount"
	// BulkRaiseManual takes a figure typed per person and ignores the current one.
	BulkRaiseManual BulkRaiseMode = "manual"
)

// BulkRaiseTarget is one employee the caller selected, plus the figure typed for them in manual mode.
type BulkRaiseTarget struct {
	EmployeesID uint
	// NewSalary is read only in manual mode; the other modes derive it.
	NewSalary float64
}

// BulkRaiseInput is one batch: who, how much, from when, and why.
type BulkRaiseInput struct {
	Mode          BulkRaiseMode
	Value         float64
	EffectiveDate string
	Reason        string
	Targets       []BulkRaiseTarget
}

// BulkRaiseRow is what the preview says about one employee.
type BulkRaiseRow struct {
	EmployeesID   uint    `json:"employees_id"`
	EmployeeName  string  `json:"employee_name"`
	EmployeesCode string  `json:"employees_code"`
	CompaniesID   uint    `json:"companies_id"`
	CurrentSalary float64 `json:"current_salary"`
	NewSalary     float64 `json:"new_salary"`
	// Blocked says this employee cannot be raised on this date, and Blocker says why in words the operator
	// can act on. A batch with any blocked row is refused whole rather than applied in part.
	Blocked bool   `json:"blocked"`
	Blocker string `json:"blocker"`
}

// BulkRaisePreview is the answer to "what would happen", with the count the screen leads on.
type BulkRaisePreview struct {
	Rows         []BulkRaiseRow `json:"rows"`
	BlockedCount int            `json:"blocked_count"`
}

// bulkPerson is one selected employee, resolved and authorized.
type bulkPerson struct {
	employee models.Employees
	target   BulkRaiseTarget
}

// roundMoney puts a figure on the two decimals the column stores, so the preview shows what will be saved
// rather than a number that is quietly rounded on the way in.
func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

// newSalaryFor works out what one employee would be paid under this batch.
func newSalaryFor(input BulkRaiseInput, target BulkRaiseTarget, current float64) float64 {
	switch input.Mode {
	case BulkRaisePercent:
		return roundMoney(current * (1 + input.Value/100))
	case BulkRaiseAmount:
		return roundMoney(current + input.Value)
	default:
		return roundMoney(target.NewSalary)
	}
}

// validateBulkInput checks the shape of the request before any employee is looked at, and returns the reason
// the batch will be recorded under.
func validateBulkInput(input BulkRaiseInput) (string, error) {
	if len(input.Targets) == 0 {
		return "", ErrBulkNoEmployees
	}
	switch input.Mode {
	case BulkRaisePercent, BulkRaiseAmount:
		if validateSalaryAmount(input.Value, 0.01) != nil {
			return "", ErrBulkValueInvalid
		}
	case BulkRaiseManual:
	default:
		return "", ErrBulkModeInvalid
	}
	return cleanReason(input.Reason)
}

// resolveBulkPeople turns the selected ids into employees the caller is allowed to touch. It runs outside the
// transaction, so the authorization reads do not sit inside the lock the writes hold.
func (s *SalariesStructureService) resolveBulkPeople(scope repository.AccessScope, targets []BulkRaiseTarget) ([]bulkPerson, error) {
	people := make([]bulkPerson, 0, len(targets))
	for _, target := range targets {
		employee, err := s.employees.FindBriefScoped(scope, target.EmployeesID)
		if err != nil {
			return nil, ErrSalaryEmployeeTarget
		}
		if _, ok := scope.ResolveCompanyID(employee.CompaniesID); !ok {
			return nil, ErrSalaryEmployeeTarget
		}
		if err := requireActiveEmployee(employee); err != nil {
			return nil, err
		}
		people = append(people, bulkPerson{employee: employee, target: target})
	}
	return people, nil
}

// bulkRows works out what the batch would do to each person, and why any of them cannot be done. It takes the
// repository rather than reaching for the service's own, so the preview can run it unlocked and the apply can
// run the very same rules again inside its transaction.
func bulkRows(
	repo repository.SalariesStructureRepository,
	input BulkRaiseInput,
	effectiveDate time.Time,
	people []bulkPerson,
) (BulkRaisePreview, error) {
	rows := make([]BulkRaiseRow, 0, len(people))
	blocked := 0
	// One lookup per company rather than per employee: a batch is normally one company's whole payroll.
	closedByCompany := map[uint]bool{}

	for _, person := range people {
		records, err := repo.ListAllRecordsUnscoped(person.target.EmployeesID, person.employee.CompaniesID)
		if err != nil {
			return BulkRaisePreview{}, err
		}

		row := BulkRaiseRow{
			EmployeesID:   person.target.EmployeesID,
			EmployeeName:  person.employee.FirstName + " " + person.employee.LastName,
			EmployeesCode: person.employee.EmployeesCode,
			CompaniesID:   person.employee.CompaniesID,
		}
		if current, ok := currentRecord(records); ok {
			row.CurrentSalary = current.BaseSalaries
		}

		block := func(why string) {
			if row.Blocked {
				return
			}
			row.Blocked, row.Blocker = true, why
			blocked++
		}

		switch {
		case row.CurrentSalary == 0 && input.Mode != BulkRaiseManual:
			block("No salary to raise from — enter a figure for this person instead")
		case input.Mode == BulkRaiseManual && roundMoney(person.target.NewSalary) <= 0:
			block("No new figure entered")
		default:
			row.NewSalary = newSalaryFor(input, person.target, row.CurrentSalary)
		}

		if !row.Blocked && validateSalaryAmount(row.NewSalary, 0.01) != nil {
			row.NewSalary = 0
			block("New salary exceeds the supported range")
		}
		if !row.Blocked {
			inClosed, seen := closedByCompany[person.employee.CompaniesID]
			if !seen {
				count, err := repo.CountClosedCyclesCovering(person.employee.CompaniesID, effectiveDate)
				if err != nil {
					return BulkRaisePreview{}, err
				}
				inClosed = count > 0
				closedByCompany[person.employee.CompaniesID] = inClosed
			}
			if inClosed {
				block("That date falls inside a payroll cycle that is already closed")
			}
		}

		// Reusing a date already on the timeline corrects that record — unless a closed cycle was paid from
		// it, which fixes the figure for good.
		if !row.Blocked {
			for _, record := range records {
				if record.DeletedAt != 0 || timeutil.DateKey(record.EffectiveDate) != timeutil.DateKey(effectiveDate) {
					continue
				}
				usage, err := repo.ClosedCyclesUsing([]uint{record.ID})
				if err != nil {
					return BulkRaisePreview{}, err
				}
				if len(usage[record.ID]) > 0 {
					block("Already paid from on that date — pick a later one")
				}
				break
			}
		}

		rows = append(rows, row)
	}

	// Blocked rows first: they are the reason the batch will not go through, so they are what the operator
	// needs to see without scrolling.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Blocked != rows[j].Blocked {
			return rows[i].Blocked
		}
		return rows[i].EmployeesCode < rows[j].EmployeesCode
	})
	return BulkRaisePreview{Rows: rows, BlockedCount: blocked}, nil
}

// BulkRaisePlan reports what the batch would do. It writes nothing, and the same rules run again inside the
// transaction when the batch is applied, because a preview is a statement about one moment and a cycle can
// close while the operator is reading it.
func (s *SalariesStructureService) BulkRaisePlan(scope repository.AccessScope, input BulkRaiseInput) (BulkRaisePreview, error) {
	if _, err := validateBulkInput(input); err != nil {
		return BulkRaisePreview{}, err
	}
	effectiveDate, err := timeutil.ParseDayStart(input.EffectiveDate)
	if err != nil {
		return BulkRaisePreview{}, ErrSalaryDateInvalid
	}

	people, err := s.resolveBulkPeople(scope, input.Targets)
	if err != nil {
		return BulkRaisePreview{}, err
	}
	return bulkRows(s.repo, input, effectiveDate, people)
}

// BulkRaiseApply puts the whole batch on the timeline in one transaction, or none of it. A raise that lands
// on half a department leaves nobody able to say who was paid what, so a blocked employee refuses the batch
// rather than being skipped.
func (s *SalariesStructureService) BulkRaiseApply(scope repository.AccessScope, input BulkRaiseInput) (BulkRaisePreview, error) {
	reason, err := validateBulkInput(input)
	if err != nil {
		return BulkRaisePreview{}, err
	}
	effectiveDate, err := timeutil.ParseDayStart(input.EffectiveDate)
	if err != nil {
		return BulkRaisePreview{}, ErrSalaryDateInvalid
	}

	people, err := s.resolveBulkPeople(scope, input.Targets)
	if err != nil {
		return BulkRaisePreview{}, err
	}

	// preview is returned even when the batch is refused, so the screen can say which people stopped it.
	preview := BulkRaisePreview{}
	err = s.repo.WithinTx(func(repo repository.SalariesStructureRepository) error {
		for _, companyID := range distinctCompanies(people) {
			if err := repo.LockCompanyCycles(companyID); err != nil {
				return err
			}
		}

		// The check inside the transaction is the one that counts: the preview the operator read may be
		// minutes old, and closing a cycle in between is exactly the race the lock exists for.
		fresh, err := bulkRows(repo, input, effectiveDate, people)
		if err != nil {
			return err
		}
		preview = fresh
		if fresh.BlockedCount > 0 {
			return ErrBulkBlocked
		}

		for _, person := range people {
			records, err := repo.ListAllRecordsUnscoped(person.target.EmployeesID, person.employee.CompaniesID)
			if err != nil {
				return err
			}
			current := 0.0
			if record, ok := currentRecord(records); ok {
				current = record.BaseSalaries
			}
			if err := applySalaryOn(repo, scope, salaryWrite{
				CompaniesID:   person.employee.CompaniesID,
				EmployeesID:   person.target.EmployeesID,
				BaseSalaries:  newSalaryFor(input, person.target, current),
				EffectiveDate: effectiveDate,
				Reason:        reason,
				Existing:      records,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return preview, err
}

// distinctCompanies lists the companies a batch touches, in a stable order so two batches never take the same
// locks in opposite orders and deadlock against each other.
func distinctCompanies(people []bulkPerson) []uint {
	seen := map[uint]bool{}
	ids := make([]uint, 0, len(people))
	for _, person := range people {
		if seen[person.employee.CompaniesID] {
			continue
		}
		seen[person.employee.CompaniesID] = true
		ids = append(ids, person.employee.CompaniesID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

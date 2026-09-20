package service

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Cycle-deduction rule failures.
var (
	ErrCycleDeductionCycleDenied     = apperr.Validation("Cycle not found or access denied")
	ErrCycleDeductionCycleClosedAdd  = apperr.Validation("Cannot add deductions to a closed cycle")
	ErrCycleDeductionCycleClosedEdit = apperr.Validation("Cannot update deductions in a closed cycle")
	ErrCycleDeductionCycleClosedDel  = apperr.Validation("Cannot delete deductions in a closed cycle")
	ErrCycleDeductionCompanyMismatch = apperr.Validation("Selected company does not match payroll cycle company")
	ErrCycleDeductionEmployeeCompany = apperr.Validation("Employee does not belong to payroll cycle company")
	ErrCycleDeductionTypeCompany     = apperr.Validation("Deduction type does not belong to payroll cycle company")
)

// Re-exported so handlers import a single package.
var (
	ErrCycleDeductionNotFound     = repository.ErrCycleDeductionNotFound
	ErrCycleDeductionNotDeletable = repository.ErrCycleDeductionNotDeletable
	ErrDeductionTypeMissing       = repository.ErrDeductionTypeMissing
)

// CycleDeductionService owns the rules for a one-off deduction on a payroll cycle.
type CycleDeductionService struct {
	repo           repository.CycleDeductionRepository
	cycles         repository.PayrollRunRepository
	employees      repository.EmployeesRepository
	deductionTypes repository.DeductionTypeRepository
}

// NewCycleDeductionService wires the service to repository implementations.
func NewCycleDeductionService(
	repo repository.CycleDeductionRepository,
	cycles repository.PayrollRunRepository,
	employees repository.EmployeesRepository,
	deductionTypes repository.DeductionTypeRepository,
) *CycleDeductionService {
	return &CycleDeductionService{repo: repo, cycles: cycles, employees: employees, deductionTypes: deductionTypes}
}

// openCycle resolves the cycle, translating its two failure modes into the message this endpoint uses for the action being attempted.
func (s *CycleDeductionService) openCycle(scope repository.AccessScope, cycleID uint, closedErr error) (models.PayrollCycle, error) {
	cycle, err := s.cycles.FindOpenCycleScoped(scope, cycleID)
	switch {
	case err == nil:
		cycle, err = s.cycles.LockCycleForUpdate(cycle.ID)
		if err != nil {
			return models.PayrollCycle{}, err
		}
		if cycle.IsClosed {
			return models.PayrollCycle{}, closedErr
		}
		return cycle, nil
	case errors.Is(err, repository.ErrPayrollCycleClosed):
		return models.PayrollCycle{}, closedErr
	case errors.Is(err, repository.ErrPayrollCycleMissing):
		return models.PayrollCycle{}, ErrCycleDeductionCycleDenied
	default:
		return models.PayrollCycle{}, err
	}
}

// List returns the cycle's deductions within the caller's scope.
func (s *CycleDeductionService) List(scope repository.AccessScope, cycleID uint, q repository.PageQuery) (repository.Page[models.PayrollCycleDeduction], error) {
	if cycleID == 0 {
		return repository.Page[models.PayrollCycleDeduction]{}, ErrCycleIDRequired
	}
	return s.repo.ListScopedByCycle(scope, cycleID, q)
}

// Create attaches a one-off deduction to an open cycle.
func (s *CycleDeductionService) Create(scope repository.AccessScope, input models.PayrollCycleDeduction) (models.PayrollCycleDeduction, error) {
	var saved models.PayrollCycleDeduction
	err := s.repo.WithinTx(func(repo repository.CycleDeductionRepository, cycles repository.PayrollRunRepository) error {
		txService := *s
		txService.repo, txService.cycles = repo, cycles
		var err error
		saved, err = txService.create(scope, input)
		return err
	})
	return saved, err
}

func (s *CycleDeductionService) create(scope repository.AccessScope, input models.PayrollCycleDeduction) (models.PayrollCycleDeduction, error) {
	deductionType, err := s.deductionTypes.Find(input.DeductionTypeID)
	if err != nil {
		return models.PayrollCycleDeduction{}, err
	}

	cycle, err := s.openCycle(scope, input.CycleID, ErrCycleDeductionCycleClosedAdd)
	if err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	if input.CompaniesID != 0 && input.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleDeduction{}, ErrCycleDeductionCompanyMismatch
	}
	employee, err := s.employees.FindBriefScoped(scope, input.EmployeesID)
	if err != nil || employee.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleDeduction{}, ErrCycleDeductionEmployeeCompany
	}
	if err := requireActiveEmployee(employee); err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	if deductionType.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleDeduction{}, ErrCycleDeductionTypeCompany
	}

	if err := validateCycleEmployeeSalary(s.cycles, cycle, input.EmployeesID); err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	item := models.PayrollCycleDeduction{
		CycleID:         input.CycleID,
		CompaniesID:     cycle.CompaniesID,
		EmployeesID:     input.EmployeesID,
		DeductionTypeID: input.DeductionTypeID,
		Amount:          input.Amount,
	}
	audit, err := masterAudit(scope, models.AuditEventCycleDeductionCreated, "Cycle deduction created", "cycle_deduction", item.CompaniesID, &item.ID, nil, cycleDeductionAuditData(item))
	if err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	if err := s.repo.Create(&item, &audit); err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	return s.repo.FindDetail(item.ID)
}

// Update rewrites a one-off deduction, which is only possible while its cycle is open.
func (s *CycleDeductionService) Update(scope repository.AccessScope, id uint, input models.PayrollCycleDeduction) (models.PayrollCycleDeduction, error) {
	var saved models.PayrollCycleDeduction
	err := s.repo.WithinTx(func(repo repository.CycleDeductionRepository, cycles repository.PayrollRunRepository) error {
		txService := *s
		txService.repo, txService.cycles = repo, cycles
		var err error
		saved, err = txService.update(scope, id, input)
		return err
	})
	return saved, err
}

func (s *CycleDeductionService) update(scope repository.AccessScope, id uint, input models.PayrollCycleDeduction) (models.PayrollCycleDeduction, error) {
	item, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.PayrollCycleDeduction{}, err
	}

	cycle, err := s.openCycle(scope, item.CycleID, ErrCycleDeductionCycleClosedEdit)
	if err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	if input.CompaniesID != 0 && input.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleDeduction{}, ErrCycleDeductionCompanyMismatch
	}

	// On update a missing deduction type and one from another company answer the same message, which is how this endpoint has always behaved.
	deductionType, err := s.deductionTypes.Find(input.DeductionTypeID)
	if err != nil || deductionType.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleDeduction{}, ErrCycleDeductionTypeCompany
	}
	employee, err := s.employees.FindBriefScoped(scope, input.EmployeesID)
	if err != nil || employee.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleDeduction{}, ErrCycleDeductionEmployeeCompany
	}

	if err := requireActiveEmployee(employee); err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	if err := validateCycleEmployeeSalary(s.cycles, cycle, input.EmployeesID); err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	before := cycleDeductionAuditData(item)
	updated := item
	updated.CompaniesID, updated.EmployeesID, updated.DeductionTypeID = cycle.CompaniesID, input.EmployeesID, input.DeductionTypeID
	fields := map[string]any{
		"amount":            input.Amount,
		"companies_id":      cycle.CompaniesID,
		"deduction_type_id": input.DeductionTypeID,
		"employees_id":      input.EmployeesID,
	}
	audit, err := masterAudit(scope, models.AuditEventCycleDeductionUpdated, "Cycle deduction updated", "cycle_deduction", cycle.CompaniesID, &item.ID, before, cycleDeductionAuditData(updated))
	if err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	if err := s.repo.UpdateFields(&item, fields, &audit); err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	return s.repo.FindDetail(item.ID)
}

// Delete removes a one-off deduction, which is only possible while its cycle is open.
func (s *CycleDeductionService) Delete(scope repository.AccessScope, id uint) error {
	return s.repo.WithinTx(func(repo repository.CycleDeductionRepository, cycles repository.PayrollRunRepository) error {
		txService := *s
		txService.repo, txService.cycles = repo, cycles
		return txService.delete(scope, id)
	})
}

func (s *CycleDeductionService) delete(scope repository.AccessScope, id uint) error {
	item, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return ErrCycleDeductionNotDeletable
	}
	if _, err := s.openCycle(scope, item.CycleID, ErrCycleDeductionCycleClosedDel); err != nil {
		return err
	}
	audit, err := masterAudit(scope, models.AuditEventCycleDeductionDeleted, "Cycle deduction deleted", "cycle_deduction", item.CompaniesID, &item.ID, cycleDeductionAuditData(item), cycleDeductionAuditData(item))
	if err != nil {
		return err
	}
	return s.repo.Delete(&item, &audit)
}

func cycleDeductionAuditData(value models.PayrollCycleDeduction) map[string]any {
	return map[string]any{"cycle_id": value.CycleID, "employees_id": value.EmployeesID, "deduction_type_id": value.DeductionTypeID}
}

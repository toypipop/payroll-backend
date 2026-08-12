package service

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Cycle-deduction rule failures. This domain answers 400 for every rule, including
// the cycle checks, which is what its callers already saw.
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

// CycleDeductionService owns the rules for a one-off deduction on a payroll cycle: the
// cycle must be open and in reach, and the employee and deduction type must both
// belong to that cycle's company.
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

// openCycle resolves the cycle, translating its two failure modes into the message
// this endpoint uses for the action being attempted.
func (s *CycleDeductionService) openCycle(scope repository.AccessScope, cycleID uint, closedErr error) (models.PayrollCycle, error) {
	cycle, err := s.cycles.FindOpenCycleScoped(scope, cycleID)
	switch {
	case err == nil:
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
func (s *CycleDeductionService) List(scope repository.AccessScope, cycleID uint) ([]models.PayrollCycleDeduction, error) {
	if cycleID == 0 {
		return nil, ErrCycleIDRequired
	}
	return s.repo.ListScopedByCycle(scope, cycleID)
}

// Create attaches a one-off deduction to an open cycle.
func (s *CycleDeductionService) Create(scope repository.AccessScope, input models.PayrollCycleDeduction) (models.PayrollCycleDeduction, error) {
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
	if deductionType.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleDeduction{}, ErrCycleDeductionTypeCompany
	}

	item := models.PayrollCycleDeduction{
		CycleID:         input.CycleID,
		CompaniesID:     cycle.CompaniesID,
		EmployeesID:     input.EmployeesID,
		DeductionTypeID: input.DeductionTypeID,
		Amount:          input.Amount,
	}
	if err := s.repo.Create(&item); err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	return s.repo.FindDetail(item.ID)
}

// Update rewrites a one-off deduction, which is only possible while its cycle is open.
func (s *CycleDeductionService) Update(scope repository.AccessScope, id uint, input models.PayrollCycleDeduction) (models.PayrollCycleDeduction, error) {
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

	// On update a missing deduction type and one from another company answer the same
	// message, which is how this endpoint has always behaved.
	deductionType, err := s.deductionTypes.Find(input.DeductionTypeID)
	if err != nil || deductionType.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleDeduction{}, ErrCycleDeductionTypeCompany
	}
	employee, err := s.employees.FindBriefScoped(scope, input.EmployeesID)
	if err != nil || employee.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleDeduction{}, ErrCycleDeductionEmployeeCompany
	}

	fields := map[string]any{
		"amount":            input.Amount,
		"companies_id":      cycle.CompaniesID,
		"deduction_type_id": input.DeductionTypeID,
		"employees_id":      input.EmployeesID,
	}
	if err := s.repo.UpdateFields(&item, fields); err != nil {
		return models.PayrollCycleDeduction{}, err
	}
	return s.repo.FindDetail(item.ID)
}

// Delete removes a one-off deduction, which is only possible while its cycle is open.
func (s *CycleDeductionService) Delete(scope repository.AccessScope, id uint) error {
	item, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return ErrCycleDeductionNotDeletable
	}
	if _, err := s.openCycle(scope, item.CycleID, ErrCycleDeductionCycleClosedDel); err != nil {
		return err
	}
	return s.repo.Delete(&item)
}

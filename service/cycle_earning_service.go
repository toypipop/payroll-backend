package service

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Cycle-earning rule failures.
var (
	ErrCycleIDRequired             = apperr.Validation("cycle_id query param required")
	ErrCycleEarningCycleDenied     = apperr.Validation("Cycle not found or access denied")
	ErrCycleEarningCycleClosedAdd  = apperr.Validation("Cannot add earnings to a closed cycle")
	ErrCycleEarningCycleClosedEdit = apperr.Validation("Cannot update earnings in a closed cycle")
	ErrCycleEarningCycleClosedDel  = apperr.Validation("Cannot delete earnings in a closed cycle")
	ErrCycleEarningCompanyMismatch = apperr.Validation("Selected company does not match payroll cycle company")
	ErrCycleEarningEmployeeCompany = apperr.Validation("Employee does not belong to payroll cycle company")
	ErrCycleEarningTypeCompany     = apperr.Validation("Earning type does not belong to payroll cycle company")
)

// Re-exported so handlers import a single package.
var (
	ErrCycleEarningNotFound     = repository.ErrCycleEarningNotFound
	ErrCycleEarningNotDeletable = repository.ErrCycleEarningNotDeletable
	ErrEarningTypeMissing       = repository.ErrEarningTypeMissing
)

// CycleEarningService owns the rules for a one-off earning on a payroll cycle.
type CycleEarningService struct {
	repo         repository.CycleEarningRepository
	cycles       repository.PayrollRunRepository
	employees    repository.EmployeesRepository
	earningTypes repository.EarningTypeRepository
}

// NewCycleEarningService wires the service to repository implementations.
func NewCycleEarningService(
	repo repository.CycleEarningRepository,
	cycles repository.PayrollRunRepository,
	employees repository.EmployeesRepository,
	earningTypes repository.EarningTypeRepository,
) *CycleEarningService {
	return &CycleEarningService{repo: repo, cycles: cycles, employees: employees, earningTypes: earningTypes}
}

// openCycle resolves the cycle, translating its two failure modes into the message this endpoint uses for the action being attempted.
func (s *CycleEarningService) openCycle(scope repository.AccessScope, cycleID uint, closedErr error) (models.PayrollCycle, error) {
	cycle, err := s.cycles.FindOpenCycleScoped(scope, cycleID)
	switch {
	case err == nil:
		return cycle, nil
	case errors.Is(err, repository.ErrPayrollCycleClosed):
		return models.PayrollCycle{}, closedErr
	case errors.Is(err, repository.ErrPayrollCycleMissing):
		return models.PayrollCycle{}, ErrCycleEarningCycleDenied
	default:
		return models.PayrollCycle{}, err
	}
}

// List returns the cycle's earnings within the caller's scope.
func (s *CycleEarningService) List(scope repository.AccessScope, cycleID uint) ([]models.PayrollCycleEarning, error) {
	if cycleID == 0 {
		return nil, ErrCycleIDRequired
	}
	return s.repo.ListScopedByCycle(scope, cycleID)
}

// validateAgainstCycle checks the company, employee and earning type against the cycle.
func (s *CycleEarningService) validateAgainstCycle(scope repository.AccessScope, cycle models.PayrollCycle, companiesID uint, employeeID uint, earningType models.EarningType) error {
	if companiesID != 0 && companiesID != cycle.CompaniesID {
		return ErrCycleEarningCompanyMismatch
	}
	employee, err := s.employees.FindBriefScoped(scope, employeeID)
	if err != nil || employee.CompaniesID != cycle.CompaniesID {
		return ErrCycleEarningEmployeeCompany
	}
	if earningType.CompaniesID != cycle.CompaniesID {
		return ErrCycleEarningTypeCompany
	}
	return nil
}

// Create attaches a one-off earning to an open cycle.
func (s *CycleEarningService) Create(scope repository.AccessScope, input models.PayrollCycleEarning) (models.PayrollCycleEarning, error) {
	earningType, err := s.earningTypes.Find(input.EarningTypeID)
	if err != nil {
		return models.PayrollCycleEarning{}, err
	}

	cycle, err := s.openCycle(scope, input.CycleID, ErrCycleEarningCycleClosedAdd)
	if err != nil {
		return models.PayrollCycleEarning{}, err
	}
	if err := s.validateAgainstCycle(scope, cycle, input.CompaniesID, input.EmployeesID, earningType); err != nil {
		return models.PayrollCycleEarning{}, err
	}

	item := models.PayrollCycleEarning{
		CycleID:       input.CycleID,
		CompaniesID:   cycle.CompaniesID,
		EmployeesID:   input.EmployeesID,
		EarningTypeID: input.EarningTypeID,
		Amount:        input.Amount,
	}
	if err := s.repo.Create(&item); err != nil {
		return models.PayrollCycleEarning{}, err
	}
	return s.repo.FindDetail(item.ID)
}

// Update rewrites a one-off earning, which is only possible while its cycle is open.
func (s *CycleEarningService) Update(scope repository.AccessScope, id uint, input models.PayrollCycleEarning) (models.PayrollCycleEarning, error) {
	item, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.PayrollCycleEarning{}, err
	}

	cycle, err := s.openCycle(scope, item.CycleID, ErrCycleEarningCycleClosedEdit)
	if err != nil {
		return models.PayrollCycleEarning{}, err
	}
	if input.CompaniesID != 0 && input.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleEarning{}, ErrCycleEarningCompanyMismatch
	}

	// On update a missing earning type and one from another company answer the same message, which is how this endpoint has always behaved.
	earningType, err := s.earningTypes.Find(input.EarningTypeID)
	if err != nil || earningType.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleEarning{}, ErrCycleEarningTypeCompany
	}
	employee, err := s.employees.FindBriefScoped(scope, input.EmployeesID)
	if err != nil || employee.CompaniesID != cycle.CompaniesID {
		return models.PayrollCycleEarning{}, ErrCycleEarningEmployeeCompany
	}

	fields := map[string]any{
		"amount":          input.Amount,
		"companies_id":    cycle.CompaniesID,
		"earning_type_id": input.EarningTypeID,
		"employees_id":    input.EmployeesID,
	}
	if err := s.repo.UpdateFields(&item, fields); err != nil {
		return models.PayrollCycleEarning{}, err
	}
	return s.repo.FindDetail(item.ID)
}

// Delete removes a one-off earning, which is only possible while its cycle is open.
func (s *CycleEarningService) Delete(scope repository.AccessScope, id uint) error {
	item, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return ErrCycleEarningNotDeletable
	}
	if _, err := s.openCycle(scope, item.CycleID, ErrCycleEarningCycleClosedDel); err != nil {
		return err
	}
	return s.repo.Delete(&item)
}

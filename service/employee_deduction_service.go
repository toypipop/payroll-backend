package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Employee-deduction rule failures.
var (
	ErrDeductionEmployeeCompany = apperr.Forbidden("Employee does not belong to selected company or access denied")
	ErrDeductionTypeCompany     = apperr.Validation("Deduction type does not belong to selected company")
)

// Re-exported so handlers import a single package.
var (
	ErrEmployeeDeductionNotFound     = repository.ErrEmployeeDeductionNotFound
	ErrEmployeeDeductionNotDeletable = repository.ErrEmployeeDeductionNotDeletable
)

// EmployeeDeductionService owns the rules for attaching a deduction to an employee:
// the employee must be one the caller can reach in the named company, and the
// deduction type must belong to that same company.
type EmployeeDeductionService struct {
	repo           repository.EmployeeDeductionRepository
	employees      repository.EmployeesRepository
	deductionTypes repository.DeductionTypeRepository
}

// NewEmployeeDeductionService wires the service to repository implementations.
func NewEmployeeDeductionService(
	repo repository.EmployeeDeductionRepository,
	employees repository.EmployeesRepository,
	deductionTypes repository.DeductionTypeRepository,
) *EmployeeDeductionService {
	return &EmployeeDeductionService{repo: repo, employees: employees, deductionTypes: deductionTypes}
}

// List returns the deductions the caller's scope covers, with relations loaded.
func (s *EmployeeDeductionService) List(scope repository.AccessScope) ([]models.EmployeeDeduction, error) {
	return s.repo.ListScoped(scope)
}

// resolveTarget confirms the employee sits in the requested company and that the
// deduction type belongs to that company too, returning the company to store.
func (s *EmployeeDeductionService) resolveTarget(scope repository.AccessScope, employeeID uint, companyID uint, deductionTypeID uint) (uint, error) {
	if employeeID == 0 || companyID == 0 {
		return 0, ErrDeductionEmployeeCompany
	}
	if _, ok := scope.ResolveCompanyID(companyID); !ok {
		return 0, ErrDeductionEmployeeCompany
	}

	employee, err := s.employees.FindBriefScoped(scope, employeeID)
	if err != nil || employee.CompaniesID != companyID {
		return 0, ErrDeductionEmployeeCompany
	}

	deductionType, err := s.deductionTypes.FindScoped(scope, deductionTypeID)
	if err != nil || deductionType.CompaniesID != employee.CompaniesID {
		return 0, ErrDeductionTypeCompany
	}
	return employee.CompaniesID, nil
}

// Create attaches a deduction type and amount to an employee.
func (s *EmployeeDeductionService) Create(scope repository.AccessScope, input models.EmployeeDeduction) (models.EmployeeDeduction, error) {
	companyID, err := s.resolveTarget(scope, input.EmployeesID, input.CompaniesID, input.DeductionTypeID)
	if err != nil {
		return models.EmployeeDeduction{}, err
	}

	deduction := models.EmployeeDeduction{
		CompaniesID:     companyID,
		EmployeesID:     input.EmployeesID,
		DeductionTypeID: input.DeductionTypeID,
		Amount:          input.Amount,
	}
	if err := s.repo.Create(&deduction); err != nil {
		return models.EmployeeDeduction{}, err
	}
	return s.repo.FindDetail(deduction.ID)
}

// Update repoints a deduction, re-running every rule against the new values.
func (s *EmployeeDeductionService) Update(scope repository.AccessScope, id uint, input models.EmployeeDeduction) (models.EmployeeDeduction, error) {
	item, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.EmployeeDeduction{}, err
	}

	companyID, err := s.resolveTarget(scope, input.EmployeesID, input.CompaniesID, input.DeductionTypeID)
	if err != nil {
		return models.EmployeeDeduction{}, err
	}

	fields := map[string]any{
		"amount":            input.Amount,
		"companies_id":      companyID,
		"deduction_type_id": input.DeductionTypeID,
		"employees_id":      input.EmployeesID,
	}
	if err := s.repo.UpdateFields(&item, fields); err != nil {
		return models.EmployeeDeduction{}, err
	}
	return s.repo.FindDetail(item.ID)
}

// Delete removes a deduction the caller's scope covers.
func (s *EmployeeDeductionService) Delete(scope repository.AccessScope, id uint) error {
	item, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return ErrEmployeeDeductionNotDeletable
	}
	return s.repo.Delete(&item)
}

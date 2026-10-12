package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Employee-earning rule failures.
var (
	ErrEarningEmployeeCompany = apperr.Forbidden("Employee does not belong to selected company or access denied")
	ErrEarningTypeCompany     = apperr.Validation("Earning type does not belong to selected company")
)

// Re-exported so handlers import a single package.
var (
	ErrEmployeeEarningNotFound     = repository.ErrEmployeeEarningNotFound
	ErrEmployeeEarningNotDeletable = repository.ErrEmployeeEarningNotDeletable
)

// EmployeeEarningService owns the rules for attaching an earning to an employee.
type EmployeeEarningService struct {
	repo         repository.EmployeeEarningRepository
	employees    repository.EmployeesRepository
	earningTypes repository.EarningTypeRepository
}

// NewEmployeeEarningService wires the service to repository implementations.
func NewEmployeeEarningService(
	repo repository.EmployeeEarningRepository,
	employees repository.EmployeesRepository,
	earningTypes repository.EarningTypeRepository,
) *EmployeeEarningService {
	return &EmployeeEarningService{repo: repo, employees: employees, earningTypes: earningTypes}
}

// List returns the earnings the caller's scope covers, with relations loaded.
func (s *EmployeeEarningService) List(scope repository.AccessScope, q repository.PageQuery) (repository.Page[models.EmployeeEarning], error) {
	return s.repo.ListScoped(scope, q)
}

// resolveTarget confirms the employee sits in the requested company and that the earning type belongs to that company too, returning the company to store.
func (s *EmployeeEarningService) resolveTarget(scope repository.AccessScope, employeeID uint, companyID uint, earningTypeID uint) (uint, error) {
	if employeeID == 0 || companyID == 0 {
		return 0, ErrEarningEmployeeCompany
	}
	if _, ok := scope.ResolveCompanyID(companyID); !ok {
		return 0, ErrEarningEmployeeCompany
	}

	employee, err := s.employees.FindBriefScoped(scope, employeeID)
	if err != nil || employee.CompaniesID != companyID {
		return 0, ErrEarningEmployeeCompany
	}
	if err := requireActiveEmployee(employee); err != nil {
		return 0, err
	}

	earningType, err := s.earningTypes.FindScoped(scope, earningTypeID)
	if err != nil || earningType.CompaniesID != employee.CompaniesID {
		return 0, ErrEarningTypeCompany
	}
	return employee.CompaniesID, nil
}

// Create attaches an earning type and amount to an employee.
func (s *EmployeeEarningService) Create(scope repository.AccessScope, input models.EmployeeEarning) (models.EmployeeEarning, error) {
	companyID, err := s.resolveTarget(scope, input.EmployeesID, input.CompaniesID, input.EarningTypeID)
	if err != nil {
		return models.EmployeeEarning{}, err
	}

	earning := models.EmployeeEarning{
		CompaniesID:   companyID,
		EmployeesID:   input.EmployeesID,
		EarningTypeID: input.EarningTypeID,
		Amount:        input.Amount,
	}
	audit, err := masterAudit(scope, models.AuditEventEmployeeEarningCreated, "Employee earning created", "employee_earning", earning.CompaniesID, &earning.ID, nil, employeeEarningAuditData(earning))
	if err != nil {
		return models.EmployeeEarning{}, err
	}
	if err := s.repo.Create(&earning, &audit); err != nil {
		return models.EmployeeEarning{}, err
	}
	return s.repo.FindDetail(earning.ID)
}

// Update repoints an earning, re-running every rule against the new values.
func (s *EmployeeEarningService) Update(scope repository.AccessScope, id uint, input models.EmployeeEarning) (models.EmployeeEarning, error) {
	item, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.EmployeeEarning{}, err
	}
	if err := requireSameCompany(item.CompaniesID, input.CompaniesID); err != nil {
		return models.EmployeeEarning{}, err
	}

	companyID, err := s.resolveTarget(scope, input.EmployeesID, input.CompaniesID, input.EarningTypeID)
	if err != nil {
		return models.EmployeeEarning{}, err
	}

	before := employeeEarningAuditData(item)
	updated := item
	updated.CompaniesID, updated.EmployeesID, updated.EarningTypeID = companyID, input.EmployeesID, input.EarningTypeID
	fields := map[string]any{
		"amount":          input.Amount,
		"companies_id":    companyID,
		"earning_type_id": input.EarningTypeID,
		"employees_id":    input.EmployeesID,
	}
	audit, err := masterAudit(scope, models.AuditEventEmployeeEarningUpdated, "Employee earning updated", "employee_earning", companyID, &item.ID, before, employeeEarningAuditData(updated))
	if err != nil {
		return models.EmployeeEarning{}, err
	}
	if err := s.repo.UpdateFields(&item, fields, &audit); err != nil {
		return models.EmployeeEarning{}, err
	}
	return s.repo.FindDetail(item.ID)
}

// Delete removes an earning the caller's scope covers.
func (s *EmployeeEarningService) Delete(scope repository.AccessScope, id uint) error {
	item, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return ErrEmployeeEarningNotDeletable
	}
	audit, err := masterAudit(scope, models.AuditEventEmployeeEarningDeleted, "Employee earning deleted", "employee_earning", item.CompaniesID, &item.ID, employeeEarningAuditData(item), employeeEarningAuditData(item))
	if err != nil {
		return err
	}
	return s.repo.Delete(&item, &audit)
}

func employeeEarningAuditData(value models.EmployeeEarning) map[string]any {
	return map[string]any{"employees_id": value.EmployeesID, "earning_type_id": value.EarningTypeID}
}

package service

import (
	"errors"
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost matches what the login path expects to verify against.
const bcryptCost = 12

// Business-rule failures; the kind decides the status, so nothing here names an HTTP code.
var (
	ErrCompanyAccessDenied = apperr.Forbidden("Access denied for this company")
	ErrRoleCompanyMismatch = apperr.Validation("Role does not belong to selected company")
	ErrPasswordHash        = errors.New("could not secure the password")
)

// ErrEmployeeNotFound is re-exported so handlers import a single package.
var ErrEmployeeNotFound = repository.ErrEmployeeNotFound

// EmployeesService owns the employee business rules: which company a record may
// belong to, that a role must match that company, and how credentials are
// stored. It talks to persistence only through EmployeesRepository.
type EmployeesService struct {
	repo repository.EmployeesRepository
}

// NewEmployeesService wires the service to a repository implementation.
func NewEmployeesService(repo repository.EmployeesRepository) *EmployeesService {
	return &EmployeesService{repo: repo}
}

func (s *EmployeesService) List(scope repository.AccessScope) ([]models.Employees, error) {
	return s.repo.ListScoped(scope)
}

// assertRoleBelongsTo enforces the rule that a role is company-owned: an
// employee can never be given a role defined under a different company.
func (s *EmployeesService) assertRoleBelongsTo(roleID uint, companyID uint) error {
	role, err := s.repo.FindRole(roleID)
	if err != nil {
		return ErrRoleCompanyMismatch
	}
	if role.CompaniesID != companyID {
		return ErrRoleCompanyMismatch
	}
	return nil
}

// Create registers a new employee under a company the caller is authorized for.
func (s *EmployeesService) Create(scope repository.AccessScope, input models.Employees) (models.Employees, error) {
	companyID, ok := scope.ResolveCompanyID(input.CompaniesID)
	if !ok {
		return models.Employees{}, ErrCompanyAccessDenied
	}
	input.CompaniesID = companyID

	if err := s.assertRoleBelongsTo(input.RolesID, companyID); err != nil {
		return models.Employees{}, err
	}

	if input.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcryptCost)
		if err != nil {
			return models.Employees{}, ErrPasswordHash
		}
		input.Password = string(hashed)
	}

	if err := s.repo.Create(&input); err != nil {
		return models.Employees{}, err
	}
	return input, nil
}

// Update applies a partial patch. The employee's company is immutable after
// creation, so companies_id is dropped from the patch regardless of what the
// client sent — this mirrors the frontend disabling the field on edit forms.
func (s *EmployeesService) Update(scope repository.AccessScope, id uint, patch map[string]any) (models.Employees, error) {
	employee, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.Employees{}, err
	}

	delete(patch, "companies_id")

	companyID, ok := scope.ResolveCompanyID(employee.CompaniesID)
	if !ok {
		return models.Employees{}, ErrCompanyAccessDenied
	}

	nextRoleID := employee.RolesID
	if rolesID, ok := patch["roles_id"].(float64); ok {
		nextRoleID = uint(rolesID)
	}
	if err := s.assertRoleBelongsTo(nextRoleID, companyID); err != nil {
		return models.Employees{}, err
	}

	if password, ok := patch["password"].(string); ok && password != "" {
		hashed, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
		if hashErr != nil {
			return models.Employees{}, ErrPasswordHash
		}
		patch["password"] = string(hashed)
	} else {
		delete(patch, "password")
	}

	normalizeBankID(patch)

	if err := s.repo.UpdateFields(&employee, patch); err != nil {
		return models.Employees{}, err
	}
	return s.repo.FindDetail(id)
}

// normalizeBankID turns the several shapes a JSON client can send for an
// optional bank (absent, null, or 0) into either a concrete id or an explicit
// "clear this column" marker.
func normalizeBankID(patch map[string]any) {
	raw, present := patch["bank_id"]
	if !present {
		return
	}
	switch v := raw.(type) {
	case float64:
		if uint(v) == 0 {
			patch["bank_id"] = repository.ClearField
			return
		}
		patch["bank_id"] = uint(v)
	case nil:
		patch["bank_id"] = repository.ClearField
	}
}

// Delete removes an employee the caller is scoped to see.
func (s *EmployeesService) Delete(scope repository.AccessScope, id uint) error {
	employee, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(&employee)
}

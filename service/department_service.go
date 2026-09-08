package service

import (
	"strings"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Department rule failures.
var (
	ErrDepartmentNameRequired = apperr.Validation("dept_name is required")
	ErrDepartmentCategoryReq  = apperr.Validation("category_id is required")
)

// ErrDepartmentNotFound is re-exported so handlers import a single package.
var ErrDepartmentNotFound = repository.ErrDepartmentNotFound

// DepartmentService owns the department rules: which company a row may belong to, and that its company never changes afterwards.
type DepartmentService struct {
	repo repository.DepartmentRepository
}

// NewDepartmentService wires the service to a repository implementation.
func NewDepartmentService(repo repository.DepartmentRepository) *DepartmentService {
	return &DepartmentService{repo: repo}
}

// List returns the departments in the caller's authorized companies, with relations loaded.
func (s *DepartmentService) List(scope repository.AccessScope) ([]models.Department, error) {
	return s.repo.ListScoped(scope)
}

// Options returns the same rows without relations, for cross-page dropdowns.
func (s *DepartmentService) Options(scope repository.AccessScope) ([]models.Department, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Create adds a department under a company the caller is authorized for.
func (s *DepartmentService) Create(scope repository.AccessScope, input models.Department) (models.Department, error) {
	name := strings.TrimSpace(input.DeptName)
	if name == "" {
		return models.Department{}, ErrDepartmentNameRequired
	}
	if input.CategoryID == 0 {
		return models.Department{}, ErrDepartmentCategoryReq
	}

	companyID, ok := scope.ResolveCompanyID(input.CompaniesID)
	if !ok {
		return models.Department{}, ErrCompanyAccessDenied
	}

	department := models.Department{
		CompaniesID: companyID,
		DeptName:    name,
		CategoryID:  input.CategoryID,
	}
	audit, err := masterAudit(scope, models.AuditEventDepartmentCreated, "Department created", "department", department.CompaniesID, &department.ID, nil, departmentAuditData(department))
	if err != nil {
		return models.Department{}, err
	}
	if err := s.repo.Create(&department, &audit); err != nil {
		return models.Department{}, err
	}
	return department, nil
}

// Update renames a department and moves its category. The company is immutable after creation, so it is never taken from input.
func (s *DepartmentService) Update(scope repository.AccessScope, id uint, name string, categoryID uint) (models.Department, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return models.Department{}, ErrDepartmentNameRequired
	}
	if categoryID == 0 {
		return models.Department{}, ErrDepartmentCategoryReq
	}

	department, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.Department{}, err
	}

	before := departmentAuditData(department)
	department.DeptName = trimmed
	department.CategoryID = categoryID
	audit, err := masterAudit(scope, models.AuditEventDepartmentUpdated, "Department updated", "department", department.CompaniesID, &department.ID, before, departmentAuditData(department))
	if err != nil {
		return models.Department{}, err
	}
	if err := s.repo.Save(&department, &audit); err != nil {
		return models.Department{}, err
	}
	return department, nil
}

// Delete removes a department the caller is scoped to see; reference checks run in middleware before this.
func (s *DepartmentService) Delete(scope repository.AccessScope, id uint) error {
	department, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	audit, err := masterAudit(scope, models.AuditEventDepartmentDeleted, "Department deleted", "department", department.CompaniesID, &department.ID, departmentAuditData(department), departmentAuditData(department))
	if err != nil {
		return err
	}
	return s.repo.Delete(&department, &audit)
}

func departmentAuditData(department models.Department) map[string]any {
	return map[string]any{"name": department.DeptName, "category_id": department.CategoryID}
}

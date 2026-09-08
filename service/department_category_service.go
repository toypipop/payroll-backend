package service

import (
	"strings"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// ErrCategoryNameRequired means the submitted name was empty or only whitespace.
var ErrCategoryNameRequired = apperr.Validation("Category name is required")

// DepartmentCategoryService owns the rules for department categories, a list every company shares.
type DepartmentCategoryService struct {
	repo repository.DepartmentCategoryRepository
}

// NewDepartmentCategoryService wires the service to a repository implementation.
func NewDepartmentCategoryService(repo repository.DepartmentCategoryRepository) *DepartmentCategoryService {
	return &DepartmentCategoryService{repo: repo}
}

// List returns every department category, ordered by name.
func (s *DepartmentCategoryService) List() ([]models.DepartmentCategory, error) {
	return s.repo.List()
}

// Create adds a category, trimming the name so whitespace cannot slip past the unique index.
func (s *DepartmentCategoryService) Create(scope repository.AccessScope, name string) (models.DepartmentCategory, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return models.DepartmentCategory{}, ErrCategoryNameRequired
	}

	category := models.DepartmentCategory{Name: trimmed}
	audit, err := BuildAuditLog(scope, models.AuditLogInput{EventType: models.AuditEventDepartmentCategoryCreated, EntityType: "department_category", EntityID: &category.ID, Summary: "Department category created", AfterData: map[string]any{"name": category.Name}})
	if err != nil {
		return models.DepartmentCategory{}, err
	}
	if err := s.repo.Create(&category, &audit); err != nil {
		return models.DepartmentCategory{}, err
	}
	return category, nil
}

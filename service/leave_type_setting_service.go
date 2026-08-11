package service

import (
	"strings"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Leave-type rule failures.
var (
	ErrLeaveTypeNameRequired = apperr.Validation("name is required")
	ErrLeaveTypeMaxDaysRange = apperr.Validation("max_days_per_year cannot be negative")
)

// ErrLeaveTypeNotFound is re-exported so handlers import a single package.
var ErrLeaveTypeNotFound = repository.ErrLeaveTypeNotFound

// LeaveTypeSettingService owns the leave-type rules: which company a row may belong to, and that its company never changes afterwards.
type LeaveTypeSettingService struct {
	repo repository.LeaveTypeSettingRepository
}

// NewLeaveTypeSettingService wires the service to a repository implementation.
func NewLeaveTypeSettingService(repo repository.LeaveTypeSettingRepository) *LeaveTypeSettingService {
	return &LeaveTypeSettingService{repo: repo}
}

// List returns the leave types of the caller's authorized companies, with relations loaded.
func (s *LeaveTypeSettingService) List(scope repository.AccessScope) ([]models.LeaveTypeSetting, error) {
	return s.repo.ListScoped(scope)
}

// Options returns the same rows without relations, for cross-page dropdowns.
func (s *LeaveTypeSettingService) Options(scope repository.AccessScope) ([]models.LeaveTypeSetting, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Create adds a leave type under a company the caller is authorized for.
func (s *LeaveTypeSettingService) Create(scope repository.AccessScope, input models.LeaveTypeSetting) (models.LeaveTypeSetting, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.LeaveTypeSetting{}, ErrLeaveTypeNameRequired
	}
	if input.MaxDaysPerYear < 0 {
		return models.LeaveTypeSetting{}, ErrLeaveTypeMaxDaysRange
	}

	companyID, ok := scope.ResolveCompanyID(input.CompaniesID)
	if !ok {
		return models.LeaveTypeSetting{}, ErrCompanyAccessDenied
	}

	leaveType := models.LeaveTypeSetting{
		CompaniesID:    companyID,
		Name:           name,
		IsPaid:         input.IsPaid,
		MaxDaysPerYear: input.MaxDaysPerYear,
	}
	if err := s.repo.Create(&leaveType); err != nil {
		return models.LeaveTypeSetting{}, err
	}
	return leaveType, nil
}

// Update rewrites a leave type's name, paid flag and annual cap. The company is immutable after creation.
func (s *LeaveTypeSettingService) Update(scope repository.AccessScope, id uint, input models.LeaveTypeSetting) (models.LeaveTypeSetting, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.LeaveTypeSetting{}, ErrLeaveTypeNameRequired
	}
	if input.MaxDaysPerYear < 0 {
		return models.LeaveTypeSetting{}, ErrLeaveTypeMaxDaysRange
	}

	leaveType, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.LeaveTypeSetting{}, err
	}

	leaveType.Name = name
	leaveType.IsPaid = input.IsPaid
	leaveType.MaxDaysPerYear = input.MaxDaysPerYear
	if err := s.repo.Save(&leaveType); err != nil {
		return models.LeaveTypeSetting{}, err
	}
	return leaveType, nil
}

// Delete removes a leave type the caller is scoped to see.
func (s *LeaveTypeSettingService) Delete(scope repository.AccessScope, id uint) error {
	leaveType, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(&leaveType)
}

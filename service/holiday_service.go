package service

import (
	"strings"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/timeutil"
)

// Holiday rule failures.
var (
	ErrHolidayNameRequired = apperr.Validation("name is required")
	ErrHolidayDateInvalid  = apperr.Validation("Invalid date format, expected YYYY-MM-DD")
)

// ErrHolidayNotFound is re-exported so handlers import a single package.
var ErrHolidayNotFound = repository.ErrHolidayNotFound

// HolidayInput is the set of fields a caller may set on a holiday.
type HolidayInput struct {
	CompaniesID uint
	Date        string
	Name        string
	IsPaid      bool
}

// HolidayService owns the holiday rules: which company a row belongs to, and that its date is a plain calendar date.
type HolidayService struct {
	repo repository.HolidayRepository
}

// NewHolidayService wires the service to a repository implementation.
func NewHolidayService(repo repository.HolidayRepository) *HolidayService {
	return &HolidayService{repo: repo}
}

// List returns the holidays of the caller's authorized companies, earliest first.
func (s *HolidayService) List(scope repository.AccessScope) ([]models.Holiday, error) {
	return s.repo.ListScoped(scope)
}

// parseHolidayFields validates the shared name and date rules.
func parseHolidayFields(name string, date string) (string, time.Time, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", time.Time{}, ErrHolidayNameRequired
	}
	parsed, err := timeutil.ParseDayStart(date)
	if err != nil {
		return "", time.Time{}, ErrHolidayDateInvalid
	}
	return trimmed, parsed, nil
}

// Create adds a holiday under a company the caller is authorized for.
func (s *HolidayService) Create(scope repository.AccessScope, input HolidayInput) (models.Holiday, error) {
	name, date, err := parseHolidayFields(input.Name, input.Date)
	if err != nil {
		return models.Holiday{}, err
	}

	companyID, ok := scope.ResolveCompanyID(input.CompaniesID)
	if !ok {
		return models.Holiday{}, ErrCompanyAccessDenied
	}

	holiday := models.Holiday{CompaniesID: companyID, Date: date, Name: name, IsPaid: input.IsPaid}
	if err := s.repo.Create(&holiday); err != nil {
		return models.Holiday{}, err
	}
	return holiday, nil
}

// Update rewrites a holiday's date, name and paid flag. The company is immutable after creation.
func (s *HolidayService) Update(scope repository.AccessScope, id uint, input HolidayInput) (models.Holiday, error) {
	name, date, err := parseHolidayFields(input.Name, input.Date)
	if err != nil {
		return models.Holiday{}, err
	}

	holiday, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.Holiday{}, err
	}

	holiday.Date = date
	holiday.Name = name
	holiday.IsPaid = input.IsPaid
	if err := s.repo.Save(&holiday); err != nil {
		return models.Holiday{}, err
	}
	return holiday, nil
}

// Delete removes a holiday the caller is scoped to see.
func (s *HolidayService) Delete(scope repository.AccessScope, id uint) error {
	holiday, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(&holiday)
}

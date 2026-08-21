package service

import (
	"strings"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Day-off rule failures.
var (
	ErrDayOffNameRequired = apperr.Validation("name is required")
	ErrDayOffWeekdayRange = apperr.Validation("weekday must be between 0 (Sunday) and 6 (Saturday)")
)

// ErrDayOffNotFound is re-exported so handlers import a single package.
var ErrDayOffNotFound = repository.ErrDayOffNotFound

// DayOffInput is the set of fields a caller may set on a day off.
type DayOffInput struct {
	CompaniesID uint
	Weekday     int
	Name        string
	IsPaid      bool
}

// DayOffService owns the day-off rules.
type DayOffService struct {
	repo      repository.DayOffRepository
	companies repository.CompanyRepository
}

// NewDayOffService wires the service to repository implementations.
func NewDayOffService(repo repository.DayOffRepository, companies repository.CompanyRepository) *DayOffService {
	return &DayOffService{repo: repo, companies: companies}
}

// List returns the day offs of the caller's authorized companies, by weekday then name.
func (s *DayOffService) List(scope repository.AccessScope) ([]models.DayOff, error) {
	return s.repo.ListScoped(scope)
}

// Options returns the same rows without relations, for cross-page weekday defaults.
func (s *DayOffService) Options(scope repository.AccessScope) ([]models.DayOff, error) {
	return s.repo.ListMinimalScoped(scope)
}

// Companies returns the companies the caller may configure day offs for.
func (s *DayOffService) Companies(scope repository.AccessScope) ([]models.Companies, error) {
	return s.companies.ListScoped(scope)
}

// validateDayOff enforces the name and weekday rules shared by create and update.
func validateDayOff(name string, weekday int) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrDayOffNameRequired
	}
	if weekday < 0 || weekday > 6 {
		return "", ErrDayOffWeekdayRange
	}
	return trimmed, nil
}

// Create adds a day off under a company the caller is authorized for.
func (s *DayOffService) Create(scope repository.AccessScope, input DayOffInput) (models.DayOff, error) {
	name, err := validateDayOff(input.Name, input.Weekday)
	if err != nil {
		return models.DayOff{}, err
	}

	companyID, ok := scope.ResolveCompanyID(input.CompaniesID)
	if !ok {
		return models.DayOff{}, ErrCompanyAccessDenied
	}

	dayOff := models.DayOff{CompaniesID: companyID, Weekday: input.Weekday, Name: name, IsPaid: input.IsPaid}
	if err := s.repo.Create(&dayOff); err != nil {
		return models.DayOff{}, err
	}
	return dayOff, nil
}

// Update rewrites a day off's weekday, name and paid flag. The company is immutable after creation.
func (s *DayOffService) Update(scope repository.AccessScope, id uint, input DayOffInput) (models.DayOff, error) {
	name, err := validateDayOff(input.Name, input.Weekday)
	if err != nil {
		return models.DayOff{}, err
	}

	dayOff, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.DayOff{}, err
	}

	dayOff.Weekday = input.Weekday
	dayOff.Name = name
	dayOff.IsPaid = input.IsPaid
	if err := s.repo.Save(&dayOff); err != nil {
		return models.DayOff{}, err
	}
	return dayOff, nil
}

// Delete removes a day off the caller is scoped to see.
func (s *DayOffService) Delete(scope repository.AccessScope, id uint) error {
	dayOff, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(&dayOff)
}

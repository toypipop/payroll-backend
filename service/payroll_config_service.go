package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Payroll-config rule failures.
var ErrPayrollConfigCompanyDenied = apperr.Forbidden("Access denied for this Companie")

// Re-exported so handlers import a single package.
var (
	ErrPayrollCycleNotFound  = repository.ErrPayrollCycleNotFound
	ErrPayrollConfigNotFound = repository.ErrPayrollConfigNotFound
)

// CycleConfig is the configuration in force for one payroll cycle.
type CycleConfig struct {
	Config      models.PayrollConfig
	CycleID     uint
	CompaniesID uint
}

// PayrollConfigInput excludes model identity, metadata, and company associations.
type PayrollConfigInput struct {
	CompaniesID    uint    `json:"companies_id"`
	SsoRate        float64 `json:"sso_rate"`
	SsoMaxAmount   float64 `json:"sso_max_amount"`
	SsoMinSalary   float64 `json:"sso_min_salary"`
	TaxCalculation string  `json:"tax_calculation"`
}

// PayrollConfigService owns the payroll-configuration rules.
type PayrollConfigService struct {
	repo repository.PayrollConfigRepository
}

// NewPayrollConfigService wires the service to a repository implementation.
func NewPayrollConfigService(repo repository.PayrollConfigRepository) *PayrollConfigService {
	return &PayrollConfigService{repo: repo}
}

// List returns the caller's authorized companies with their current configuration.
func (s *PayrollConfigService) List(scope repository.AccessScope, q repository.PageQuery) (repository.Page[models.Companies], error) {
	return s.repo.ListCompaniesWithConfig(scope, q)
}

// Save writes the configuration for a company the caller administers, updating the existing row when there is one.
func (s *PayrollConfigService) Save(scope repository.AccessScope, input PayrollConfigInput) (models.PayrollConfig, error) {
	if !scope.CanAccessCompany(input.CompaniesID) {
		return models.PayrollConfig{}, ErrPayrollConfigCompanyDenied
	}
	for _, field := range []struct {
		name           string
		value, maximum float64
	}{
		{"SSO rate (%)", input.SsoRate, 100},
		{"Max SSO amount", input.SsoMaxAmount, maximumConfigAmount},
		{"Min salary for SSO", input.SsoMinSalary, maximumConfigAmount},
	} {
		if err := validateConfigNumber(field.name, field.value, field.maximum); err != nil {
			return models.PayrollConfig{}, err
		}
	}
	taxCalculation, err := normalizeTaxCalculation(input.TaxCalculation)
	if err != nil {
		return models.PayrollConfig{}, err
	}
	input.TaxCalculation = taxCalculation

	audit, err := masterAudit(scope, models.AuditEventPayrollConfigSaved, "Payroll configuration saved", "payroll_config", input.CompaniesID, nil, nil, map[string]any{"tax_calculation": input.TaxCalculation})
	if err != nil {
		return models.PayrollConfig{}, err
	}
	configuration := models.PayrollConfig{
		CompaniesID: input.CompaniesID, SsoRate: input.SsoRate,
		SsoMaxAmount: input.SsoMaxAmount, SsoMinSalary: input.SsoMinSalary,
		TaxCalculation: input.TaxCalculation,
	}
	if err := s.repo.UpsertForCompany(&configuration, &audit); err != nil {
		return models.PayrollConfig{}, err
	}
	return configuration, nil
}

// ForCycle returns the configuration in force for a cycle's company.
func (s *PayrollConfigService) ForCycle(scope repository.AccessScope, cycleID uint) (CycleConfig, error) {
	cycle, err := s.repo.FindCycleScoped(scope, cycleID)
	if err != nil {
		return CycleConfig{}, err
	}

	payrollConfig, err := s.repo.FindActiveForCompany(cycle.CompaniesID)
	if err != nil {
		return CycleConfig{}, err
	}

	return CycleConfig{Config: payrollConfig, CycleID: cycle.ID, CompaniesID: cycle.CompaniesID}, nil
}

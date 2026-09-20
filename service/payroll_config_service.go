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
func (s *PayrollConfigService) Save(scope repository.AccessScope, input models.PayrollConfig) (models.PayrollConfig, error) {
	if !scope.CanAccessCompany(input.CompaniesID) {
		return models.PayrollConfig{}, ErrPayrollConfigCompanyDenied
	}

	audit, err := masterAudit(scope, models.AuditEventPayrollConfigSaved, "Payroll configuration saved", "payroll_config", input.CompaniesID, nil, nil, map[string]any{"tax_calculation": input.TaxCalculation})
	if err != nil {
		return models.PayrollConfig{}, err
	}
	if err := s.repo.UpsertForCompany(&input, &audit); err != nil {
		return models.PayrollConfig{}, err
	}
	return input, nil
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

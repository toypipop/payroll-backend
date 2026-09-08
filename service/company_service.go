package service

import (
	"strings"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

// Bootstrap policy for a new company: an ADMIN role with every action on every active module except CompaniesPage.
const (
	bootstrapRoleName        = "ADMIN"
	bootstrapRoleDescription = "Company Admin - full access to all modules"
)

// ErrCompanyNameRequired means the submitted company name was empty or only whitespace.
var ErrCompanyNameRequired = apperr.Validation("name is required")

// ErrCompanyNotFound is re-exported so handlers import a single package.
var ErrCompanyNotFound = repository.ErrCompanyNotFound

// CompanyUpdate is the set of company fields a caller may rewrite.
type CompanyUpdate struct {
	Name    string
	TaxID   string
	Address string
	Phone   string
}

// CompanyService owns the company rules, including what a brand-new tenant must contain to be usable.
type CompanyService struct {
	repo repository.CompanyRepository
}

// NewCompanyService wires the service to a repository implementation.
func NewCompanyService(repo repository.CompanyRepository) *CompanyService {
	return &CompanyService{repo: repo}
}

// Options returns the companies the caller can reach, for the company dropdown every page uses.
func (s *CompanyService) Options(userID uint) ([]models.Companies, error) {
	return s.repo.ListAccessible(userID)
}

// ListAll returns every company for the management page.
func (s *CompanyService) ListAll() ([]models.Companies, error) {
	return s.repo.ListAll()
}

// Create bootstraps a usable tenant: the company, a default ADMIN role with full permissions on every active module, and company access for the creator.
func (s *CompanyService) Create(creatorID uint, input models.Companies) (models.Companies, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.Companies{}, ErrCompanyNameRequired
	}
	input.Name = name
	input.IsActive = true

	boot := repository.TenantBootstrap{
		RoleName:        bootstrapRoleName,
		RoleDescription: bootstrapRoleDescription,
		CreatorID:       creatorID,
		ExcludeModules:  []models.ModuleKey{"CompaniesPage"},
		Grant: models.RolesModuleAccess{
			CanView:   true,
			CanCreate: true,
			CanEdit:   true,
			CanDelete: true,
			DataScope: models.ScopeCompanies,
		},
	}
	if err := s.repo.CreateTenant(&input, boot); err != nil {
		return models.Companies{}, err
	}
	return input, nil
}

// Update rewrites a company's profile fields.
func (s *CompanyService) Update(id uint, input CompanyUpdate) (models.Companies, error) {
	company, err := s.repo.Find(id)
	if err != nil {
		return models.Companies{}, err
	}

	if name := strings.TrimSpace(input.Name); name != "" {
		company.Name = name
	}
	if taxID := strings.TrimSpace(input.TaxID); taxID != "" {
		company.TaxID = taxID
	}
	if address := strings.TrimSpace(input.Address); address != "" {
		company.Address = address
	}
	if phone := strings.TrimSpace(input.Phone); phone != "" {
		company.Phone = phone
	}

	if err := s.repo.Save(&company); err != nil {
		return models.Companies{}, err
	}
	return company, nil
}

// SetActive changes operational availability while retaining the company and its history.
func (s *CompanyService) SetActive(id uint, active bool) (models.Companies, error) {
	return s.repo.SetActive(id, active)
}

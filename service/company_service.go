package service

import (
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
func (s *CompanyService) Create(scope repository.AccessScope, input models.Companies) (models.Companies, error) {
	profile, err := normalizeCompanyProfile(CompanyUpdate{Name: input.Name, TaxID: input.TaxID, Address: input.Address, Phone: input.Phone})
	if err != nil {
		return models.Companies{}, err
	}
	input = models.Companies{Name: profile.Name, TaxID: profile.TaxID, Address: profile.Address, Phone: profile.Phone, IsActive: true}

	boot := repository.TenantBootstrap{
		RoleName:        bootstrapRoleName,
		RoleDescription: bootstrapRoleDescription,
		CreatorID:       scope.User.ID,
		ExcludeModules:  []models.ModuleKey{"CompaniesPage"},
		Grant: models.RolesModuleAccess{
			CanView:   true,
			CanCreate: true,
			CanEdit:   true,
			CanDelete: true,
			DataScope: models.ScopeCompanies,
		},
	}
	audit, err := BuildAuditLog(scope, models.AuditLogInput{EventType: models.AuditEventCompanyCreated, EntityType: "company", Summary: "Company created", AfterData: map[string]any{"name": input.Name}})
	if err != nil {
		return models.Companies{}, err
	}
	if err := s.repo.CreateTenant(&input, boot, &audit); err != nil {
		return models.Companies{}, err
	}
	return input, nil
}

// Update rewrites a company's profile fields.
func (s *CompanyService) Update(scope repository.AccessScope, id uint, input CompanyUpdate) (models.Companies, error) {
	company, err := s.repo.Find(id)
	if err != nil {
		return models.Companies{}, err
	}

	profile, err := normalizeCompanyProfile(input)
	if err != nil {
		return models.Companies{}, err
	}
	company.Name, company.TaxID, company.Address, company.Phone = profile.Name, profile.TaxID, profile.Address, profile.Phone

	companyID := company.ID
	audit, err := BuildAuditLog(scope, models.AuditLogInput{CompaniesID: &companyID, EntityID: &companyID, EventType: models.AuditEventCompanyUpdated, EntityType: "company", Summary: "Company updated", AfterData: map[string]any{"name": company.Name}})
	if err != nil {
		return models.Companies{}, err
	}
	if err := s.repo.Save(&company, &audit); err != nil {
		return models.Companies{}, err
	}
	return company, nil
}

// SetActive changes operational availability while retaining the company and its history.
func (s *CompanyService) SetActive(scope repository.AccessScope, id uint, active bool) (models.Companies, error) {
	audit, err := BuildAuditLog(scope, models.AuditLogInput{CompaniesID: &id, EntityID: &id, EventType: models.AuditEventCompanyActivated, EntityType: "company", Summary: "Company activated"})
	if !active {
		audit, err = BuildAuditLog(scope, models.AuditLogInput{CompaniesID: &id, EntityID: &id, EventType: models.AuditEventCompanyDeactivated, EntityType: "company", Summary: "Company deactivated"})
	}
	if err != nil {
		return models.Companies{}, err
	}
	return s.repo.SetActive(id, active, &audit)
}

func (s *CompanyService) ListPage(q repository.PageQuery, active *bool) (repository.Page[models.Companies], error) {
	return s.repo.ListPage(q, active)
}

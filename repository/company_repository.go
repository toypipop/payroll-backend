package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrCompanyNotFound means the referenced company row does not exist.
var ErrCompanyNotFound = apperr.NotFound("Companies not found")

// TenantBootstrap carries the role and grant policy to create alongside a new
// company. The values come from the service; only the atomicity lives here.
type TenantBootstrap struct {
	RoleName        string
	RoleDescription string
	CreatorID       uint
	ExcludeModules  []models.ModuleKey
	Grant           models.RolesModuleAccess
}

// CompanyRepository is the persistence boundary for the companies domain.
type CompanyRepository interface {
	ListScoped(scope AccessScope) ([]models.Companies, error)
	ListAccessible(userID uint) ([]models.Companies, error)
	ListAll() ([]models.Companies, error)
	Find(id uint) (models.Companies, error)
	Save(company *models.Companies) error
	Delete(company *models.Companies) error
	CreateTenant(company *models.Companies, boot TenantBootstrap) error
}

type gormCompanyRepository struct {
	db *gorm.DB
}

// NewCompanyRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewCompanyRepository() CompanyRepository {
	return &gormCompanyRepository{db: config.DB}
}

// ListScoped returns the companies the caller's role authorizes for the current action, lowest id first.
func (r *gormCompanyRepository) ListScoped(scope AccessScope) ([]models.Companies, error) {
	var companies []models.Companies
	err := r.db.Scopes(scope.ApplyCompany("companies.id")).Order("id ASC").Find(&companies).Error
	return companies, err
}

// ListAccessible returns the companies the caller can reach at all (home plus
// employee_company_access). This is deliberately not the authorized-company list:
// that one is narrowed to roles that can view CompaniesPage, which company-level
// ADMIN intentionally lacks, yet every role still needs the company dropdown.
func (r *gormCompanyRepository) ListAccessible(userID uint) ([]models.Companies, error) {
	var companies []models.Companies
	err := r.db.Where("id IN ?", AccessibleCompanyIDs(userID)).Order("id ASC").Find(&companies).Error
	return companies, err
}

// ListAll returns every company for the management page, newest first.
func (r *gormCompanyRepository) ListAll() ([]models.Companies, error) {
	var companies []models.Companies
	err := r.db.Order("id desc").Find(&companies).Error
	return companies, err
}

func (r *gormCompanyRepository) Find(id uint) (models.Companies, error) {
	var company models.Companies
	if err := r.db.First(&company, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Companies{}, ErrCompanyNotFound
		}
		return models.Companies{}, err
	}
	return company, nil
}

func (r *gormCompanyRepository) Save(company *models.Companies) error {
	return r.db.Save(company).Error
}

func (r *gormCompanyRepository) Delete(company *models.Companies) error {
	return r.db.Delete(company).Error
}

// CreateTenant writes the company, its bootstrap role, that role's grants on
// every active module, and the creator's access to the new company as one
// transaction, so a half-built tenant can never be left behind.
func (r *gormCompanyRepository) CreateTenant(company *models.Companies, boot TenantBootstrap) error {
	excluded := make(map[models.ModuleKey]bool, len(boot.ExcludeModules))
	for _, key := range boot.ExcludeModules {
		excluded[key] = true
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(company).Error; err != nil {
			return err
		}

		role := models.Roles{
			CompaniesID: company.ID,
			RolesName:   boot.RoleName,
			Description: boot.RoleDescription,
		}
		if err := tx.Create(&role).Error; err != nil {
			return err
		}

		var modules []models.AppModule
		if err := tx.Where("is_active = ?", true).Find(&modules).Error; err != nil {
			return err
		}

		accesses := make([]models.RolesModuleAccess, 0, len(modules))
		for _, module := range modules {
			if excluded[module.ModuleKey] {
				continue
			}
			grant := boot.Grant
			grant.CompaniesID = company.ID
			grant.RolesID = role.ID
			grant.ModuleID = module.ID
			accesses = append(accesses, grant)
		}
		if len(accesses) > 0 {
			if err := tx.Create(&accesses).Error; err != nil {
				return err
			}
		}

		if boot.CreatorID != 0 {
			creatorAccess := models.EmployeeCompanyAccess{
				EmployeeID: boot.CreatorID,
				CompanyID:  company.ID,
				RoleID:     role.ID,
				IsActive:   true,
			}
			if err := tx.Create(&creatorAccess).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

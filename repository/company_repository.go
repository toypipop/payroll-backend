package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrCompanyNotFound means the referenced company row does not exist.
var ErrCompanyNotFound = apperr.NotFound("Companies not found")

// TenantBootstrap carries the role and grant policy to create alongside a new company.
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
	SetActive(id uint, active bool) (models.Companies, error)
	CreateTenant(company *models.Companies, boot TenantBootstrap) error
}

type gormCompanyRepository struct {
	db *gorm.DB
}

// NewCompanyRepository returns the GORM-backed implementation on the given connection.
func NewCompanyRepository(db *gorm.DB) CompanyRepository {
	return &gormCompanyRepository{db: db}
}

// ListScoped returns the companies the caller's role authorizes for the current action, lowest id first.
func (r *gormCompanyRepository) ListScoped(scope AccessScope) ([]models.Companies, error) {
	var companies []models.Companies
	err := r.db.Scopes(scope.ApplyCompany("companies.id")).Where("companies.is_active = ?", true).Order("id ASC").Find(&companies).Error
	return companies, err
}

// ListAccessible returns the companies the caller can reach at all (home plus employee_company_access).
func (r *gormCompanyRepository) ListAccessible(userID uint) ([]models.Companies, error) {
	var companies []models.Companies
	err := r.db.Where("id IN ?", newGormAccessRepository(r.db).AccessibleCompanyIDs(userID)).Where("companies.is_active = ?", true).Order("id ASC").Find(&companies).Error
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
	return r.db.Transaction(func(tx *gorm.DB) error {
		current, err := lockCompany(tx, company.ID)
		if err != nil {
			return err
		}
		if !current.IsActive {
			return apperr.CompanyInactive()
		}
		// Never save a caller's stale status or associations during a profile update.
		if err := tx.Model(&current).Updates(map[string]any{
			"name": company.Name, "tax_id": company.TaxID,
			"address": company.Address, "phone": company.Phone,
		}).Error; err != nil {
			return TranslateUniqueViolation(err)
		}
		*company = current
		return nil
	})
}

func lockCompany(tx *gorm.DB, id uint) (models.Companies, error) {
	var company models.Companies
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&company, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Companies{}, ErrCompanyNotFound
		}
		return models.Companies{}, err
	}
	return company, nil
}

// SetActive serializes status and profile changes against ordinary requests' company locks.
func (r *gormCompanyRepository) SetActive(id uint, active bool) (models.Companies, error) {
	var company models.Companies
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		company, err = lockCompany(tx, id)
		if err != nil {
			return err
		}
		if company.IsActive == active {
			return nil
		}
		if !active {
			var count int64
			if err := tx.Model(&models.PayrollCycle{}).Where("companies_id = ? AND is_closed = ?", id, false).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return apperr.Conflict("Close all open payroll cycles before deactivating this company")
			}
		}
		return tx.Model(&company).Update("is_active", active).Error
	})
	return company, err
}

// CreateTenant writes the company, its bootstrap role, that role's grants and the creator's access as one transaction.
func (r *gormCompanyRepository) CreateTenant(company *models.Companies, boot TenantBootstrap) error {
	company.IsActive = true
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

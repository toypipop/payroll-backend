package repository

import (
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// CompanyRepository is the persistence boundary for the companies domain. It
// currently carries only the scoped read that other domains need; the CompaniesPage
// CRUD still lives in its handler and moves here when that domain is migrated.
type CompanyRepository interface {
	ListScoped(scope AccessScope) ([]models.Companies, error)
}

type gormCompanyRepository struct {
	db *gorm.DB
}

// NewCompanyRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewCompanyRepository() CompanyRepository {
	return &gormCompanyRepository{db: config.DB}
}

// ListScoped returns the companies the caller is authorized for, lowest id first.
func (r *gormCompanyRepository) ListScoped(scope AccessScope) ([]models.Companies, error) {
	var companies []models.Companies
	err := r.db.Scopes(scope.ApplyCompany("companies.id")).Order("id ASC").Find(&companies).Error
	return companies, err
}

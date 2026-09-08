package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrRoleAccessDenied keeps the pre-existing 403 answer for a role that is missing or out of scope, since the two are not distinguished here.
var ErrRoleAccessDenied = apperr.Forbidden("Access denied or record not found")

// RoleRepository is the persistence boundary for the roles domain.
type RoleRepository interface {
	WithinTx(func(RoleRepository) error) error
	ListScoped(scope AccessScope) ([]models.Roles, error)
	ListMinimalScoped(scope AccessScope) ([]models.Roles, error)
	FindScoped(scope AccessScope, id uint) (models.Roles, error)
	Create(role *models.Roles) error
	UpdateName(role *models.Roles, name string) error
	Delete(role *models.Roles) error
	AppendAuditLog(entry *models.AuditLog) error
}

func (r *gormRoleRepository) WithinTx(fn func(RoleRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error { return fn(&gormRoleRepository{db: tx}) })
}

type gormRoleRepository struct {
	db *gorm.DB
}

// NewRoleRepository returns the GORM-backed implementation on the given connection.
func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &gormRoleRepository{db: db}
}

// scoped filters by authorized company, the filter RolesPage declares as a company-level module.
func (r *gormRoleRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("roles.companies_id"))
}

func (r *gormRoleRepository) ListScoped(scope AccessScope) ([]models.Roles, error) {
	var roles []models.Roles
	err := r.scoped(scope).Preload("Companies").Find(&roles).Error
	return roles, err
}

// ListMinimalScoped skips the relations, for the dropdown options endpoint that needs id/name only.
func (r *gormRoleRepository) ListMinimalScoped(scope AccessScope) ([]models.Roles, error) {
	var roles []models.Roles
	err := r.scoped(scope).Find(&roles).Error
	return roles, err
}

func (r *gormRoleRepository) FindScoped(scope AccessScope, id uint) (models.Roles, error) {
	var role models.Roles
	if err := r.scoped(scope).First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Roles{}, ErrRoleAccessDenied
		}
		return models.Roles{}, err
	}
	return role, nil
}

func (r *gormRoleRepository) Create(role *models.Roles) error {
	return r.db.Create(role).Error
}

func (r *gormRoleRepository) UpdateName(role *models.Roles, name string) error {
	return r.db.Model(role).Updates(map[string]any{"roles_name": name}).Error
}

func (r *gormRoleRepository) Delete(role *models.Roles) error {
	return r.db.Delete(role).Error
}

func (r *gormRoleRepository) AppendAuditLog(entry *models.AuditLog) error {
	return r.db.Create(entry).Error
}

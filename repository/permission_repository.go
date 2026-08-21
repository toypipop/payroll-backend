package repository

import (
	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrUnknownModule marks a payload naming a module row that does not exist, or one with no capability declaration.
var ErrUnknownModule = apperr.Validation("Permission payload references an unknown module")

// PermissionRepository is the persistence boundary for the role-permission matrix.
type PermissionRepository interface {
	ListModulesOrdered() ([]models.AppModule, error)
	ListRoleAccessScoped(scope AccessScope, rolesID uint) ([]models.RolesModuleAccess, error)
	CapabilitiesFor(moduleIDs []uint) (map[uint]models.ModuleCapability, error)
	ReplaceRoleAccess(scope AccessScope, rolesID uint, permissions []models.RolesModuleAccess) error
}

type gormPermissionRepository struct {
	db *gorm.DB
}

// NewPermissionRepository returns the GORM-backed implementation on the given connection.
func NewPermissionRepository(db *gorm.DB) PermissionRepository {
	return &gormPermissionRepository{db: db}
}

// ListModulesOrdered returns the whole module tree in display order; the matrix shows every module regardless of which companies the caller administers.
func (r *gormPermissionRepository) ListModulesOrdered() ([]models.AppModule, error) {
	var modules []models.AppModule
	// Let GORM quote the reserved column name for PostgreSQL instead of hand-writing SQL.
	err := r.db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}}).Find(&modules).Error
	return modules, err
}

func (r *gormPermissionRepository) ListRoleAccessScoped(scope AccessScope, rolesID uint) ([]models.RolesModuleAccess, error) {
	var access []models.RolesModuleAccess
	err := r.db.Scopes(scope.ApplyCompany("roles_module_access.companies_id")).
		Where("roles_id = ?", rolesID).
		Find(&access).Error
	return access, err
}

// CapabilitiesFor resolves every module id to its declared capability in one query, so the write loop stays free of per-item lookups.
func (r *gormPermissionRepository) CapabilitiesFor(moduleIDs []uint) (map[uint]models.ModuleCapability, error) {
	var modules []models.AppModule
	if err := r.db.Select("id, module_key").Where("id IN ?", moduleIDs).Find(&modules).Error; err != nil {
		return nil, err
	}

	capabilities := make(map[uint]models.ModuleCapability, len(modules))
	for _, module := range modules {
		capability, ok := models.CapabilityFor(module.ModuleKey)
		if !ok {
			return nil, ErrUnknownModule
		}
		capabilities[module.ID] = capability
	}

	for _, id := range moduleIDs {
		if _, ok := capabilities[id]; !ok {
			return nil, ErrUnknownModule
		}
	}
	return capabilities, nil
}

// ReplaceRoleAccess swaps a role's permission rows in one transaction.
func (r *gormPermissionRepository) ReplaceRoleAccess(scope AccessScope, rolesID uint, permissions []models.RolesModuleAccess) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Scopes(scope.ApplyCompany("roles_module_access.companies_id")).
			Where("roles_id = ?", rolesID).
			Delete(&models.RolesModuleAccess{}).Error; err != nil {
			return err
		}
		if len(permissions) == 0 {
			return nil
		}
		return tx.Create(&permissions).Error
	})
}

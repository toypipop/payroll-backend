package repository

import (
	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrUnknownModule marks a payload naming a module row that does not exist, or one with no capability declaration.
var ErrUnknownModule = apperr.Validation("Permission payload references an unknown module")

// ModuleCapabilities is the capability lookup for one permission payload. A category module is a
// heading in the tree rather than a page: it has no routes, so it has no capability and no permission
// row, and it must not be mistaken for an unknown id.
type ModuleCapabilities struct {
	ByModule   map[uint]models.ModuleCapability
	Categories map[uint]bool
}

// PermissionRepository is the persistence boundary for the role-permission matrix.
type PermissionRepository interface {
	ListModulesOrdered() ([]models.AppModule, error)
	ListRoleAccessScoped(scope AccessScope, rolesID uint) ([]models.RolesModuleAccess, error)
	CapabilitiesFor(moduleIDs []uint) (ModuleCapabilities, error)
	ReplaceRoleAccess(scope AccessScope, rolesID uint, permissions []models.RolesModuleAccess, audit *models.AuditLog) error
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
func (r *gormPermissionRepository) CapabilitiesFor(moduleIDs []uint) (ModuleCapabilities, error) {
	var modules []models.AppModule
	if err := r.db.Select("id, module_key, is_category").Where("id IN ?", moduleIDs).Find(&modules).Error; err != nil {
		return ModuleCapabilities{}, err
	}

	resolved := ModuleCapabilities{
		ByModule:   make(map[uint]models.ModuleCapability, len(modules)),
		Categories: make(map[uint]bool),
	}
	for _, module := range modules {
		if module.IsCategory {
			resolved.Categories[module.ID] = true
			continue
		}

		capability, ok := models.CapabilityFor(module.ModuleKey)
		if !ok {
			return ModuleCapabilities{}, ErrUnknownModule
		}
		resolved.ByModule[module.ID] = capability
	}

	for _, id := range moduleIDs {
		if _, ok := resolved.ByModule[id]; ok {
			continue
		}
		if resolved.Categories[id] {
			continue
		}
		return ModuleCapabilities{}, ErrUnknownModule
	}
	return resolved, nil
}

// ReplaceRoleAccess swaps a role's permission rows in one transaction.
func (r *gormPermissionRepository) ReplaceRoleAccess(scope AccessScope, rolesID uint, permissions []models.RolesModuleAccess, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Serialize saves and snapshot the previous grants in the mutation transaction.
		var role models.Roles
		if err := tx.Scopes(scope.ApplyCompany("roles.companies_id")).Clauses(clause.Locking{Strength: "UPDATE"}).First(&role, rolesID).Error; err != nil {
			return err
		}
		var previous []models.RolesModuleAccess
		if err := tx.Scopes(scope.ApplyCompany("roles_module_access.companies_id")).Where("roles_id = ?", rolesID).Find(&previous).Error; err != nil {
			return err
		}
		var entry models.AuditLog
		if audit != nil {
			validated, err := models.NewAuditLogInput(models.AuditLogInput{EventType: audit.EventType, BeforeData: models.PermissionAuditData(previous)})
			if err != nil {
				return err
			}
			entry = *audit
			entry.BeforeData = validated.BeforeData
		}
		if err := tx.Scopes(scope.ApplyCompany("roles_module_access.companies_id")).
			Where("roles_id = ?", rolesID).
			Delete(&models.RolesModuleAccess{}).Error; err != nil {
			return err
		}
		if len(permissions) > 0 {
			if err := tx.Create(&permissions).Error; err != nil {
				return err
			}
		}
		if audit != nil {
			return tx.Create(&entry).Error
		}
		return nil
	})
}

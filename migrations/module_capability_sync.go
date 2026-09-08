package migrations

import (
	"fmt"
	"log"
	"reflect"

	"payroll-backend/models"

	"gorm.io/gorm"
)

// SyncModuleCapabilities rewrites every module's capability columns from the moduleCapabilities registry in models/module_capability.go.
func SyncModuleCapabilities(db *gorm.DB) error {
	var modules []models.AppModule
	if err := db.Find(&modules).Error; err != nil {
		return fmt.Errorf("load app modules: %w", err)
	}

	var undeclared []string
	updated := 0

	for i := range modules {
		module := modules[i]
		if module.IsCategory {
			continue
		}

		capability, ok := models.CapabilityFor(module.ModuleKey)
		if !ok {
			if module.IsActive {
				undeclared = append(undeclared, string(module.ModuleKey))
			}
			continue
		}

		scopes := capability.AllowedScopes()
		unchanged := module.ScopeLevel == capability.ScopeLevel &&
			module.ActionsAtomic == capability.ActionsAtomic &&
			reflect.DeepEqual(module.AllowedActions, capability.AllowedActions) &&
			reflect.DeepEqual(module.AllowedScopes, scopes)
		if unchanged {
			continue
		}

		// Must be a struct update, not a map: GORM applies the serializer:json tag on AllowedActions/AllowedScopes only when it knows which model field it is writing.
		if err := db.Model(&models.AppModule{}).
			Where("id = ?", module.ID).
			Select("scope_level", "allowed_actions", "allowed_scopes", "actions_atomic").
			Updates(models.AppModule{
				ScopeLevel:     capability.ScopeLevel,
				AllowedActions: capability.AllowedActions,
				AllowedScopes:  scopes,
				ActionsAtomic:  capability.ActionsAtomic,
			}).Error; err != nil {
			return fmt.Errorf("sync capabilities for %s: %w", module.ModuleKey, err)
		}
		updated++
	}

	if len(undeclared) > 0 {
		log.Fatalf(
			"FATAL: active modules with no entry in models/module_capability.go: %v\n"+
				"Declare each one's ScopeLevel (which Apply*Scopes its handler uses) and AllowedActions "+
				"(the action strings its routes pass to CheckPermission) before starting the server.",
			undeclared,
		)
	}

	fmt.Printf("Module capabilities synced (%d updated)\n", updated)
	return nil
}

// PruneStaleRoleScopes rewrites saved permission rows whose DataScope a company-level module cannot honour.
func PruneStaleRoleScopes(db *gorm.DB) error {
	// All was retired in favour of explicit employee-company access. Convert legacy rows before resolving
	// request scopes so no saved permission can bypass that access list during a rolling deployment.
	if err := db.Model(&models.RolesModuleAccess{}).
		Where("data_scope = ?", "All").
		Update("data_scope", models.ScopeCompanies).Error; err != nil {
		return fmt.Errorf("replace retired All scopes: %w", err)
	}

	var modules []models.AppModule
	if err := db.Where("is_category = ?", false).Find(&modules).Error; err != nil {
		return fmt.Errorf("load app modules: %w", err)
	}

	fixed := int64(0)
	for i := range modules {
		capability, ok := models.CapabilityFor(modules[i].ModuleKey)
		if !ok || capability.ScopeLevel == models.ScopeLevelEmployee {
			continue
		}

		result := db.Model(&models.RolesModuleAccess{}).
			Where("module_id = ? AND data_scope <> ?", modules[i].ID, models.ScopeCompanies).
			Update("data_scope", models.ScopeCompanies)
		if result.Error != nil {
			return fmt.Errorf("prune scopes for %s: %w", modules[i].ModuleKey, result.Error)
		}
		fixed += result.RowsAffected
	}

	if fixed > 0 {
		fmt.Printf("Normalised %d permission rows to Companies scope on company-level modules\n", fixed)
	}
	return nil
}

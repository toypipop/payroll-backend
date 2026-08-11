package migrations

import (
	"errors"
	"fmt"
	"log"
	"reflect"

	"payroll-backend/models"

	"gorm.io/gorm"
)

// SyncModuleCapabilities rewrites every module's capability columns from the
// moduleCapabilities registry in models/module_capability.go.
//
// The registry is the source of truth; these columns are only a projection of
// it that the frontend can read over /modules. Running this on every boot means
// the two cannot drift: editing the columns directly has no lasting effect, and
// a deploy never depends on someone remembering to reseed.
//
// A seeded, active, non-category module with no registry entry is a genuine
// mistake — someone added a module without declaring what the backend can
// enforce for it — so this fails the boot rather than guessing a default.
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

		// Must be a struct update, not a map: GORM applies the serializer:json tag
		// on AllowedActions/AllowedScopes only when it knows which model field it
		// is writing. A map hands the raw []string to the driver, which cannot type
		// a text parameter from it (SQLSTATE 42P18). Select names the columns so the
		// false value of ActionsAtomic is still written instead of skipped as a zero.
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

// RemoveDashboardModule drops the DashboardPage module and its permission
// rows from databases seeded before Dashboard stopped being permission-
// controlled. Seed only ever creates or updates module rows, so removing the
// entry from the seed list leaves existing rows behind — and once DashboardPage
// left the capability registry, SyncModuleCapabilities would refuse to boot
// over that leftover row. Must therefore run before the capability sync.
//
// Nothing is lost: /dashboard is a hardcoded route that "/" redirects to, so
// the module row only ever controlled whether the sidebar link was drawn.
func RemoveDashboardModule(db *gorm.DB) error {
	var module models.AppModule
	err := db.Where("module_key = ?", "DashboardPage").First(&module).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("look up DashboardPage module: %w", err)
	}

	if err := db.Unscoped().
		Where("module_id = ?", module.ID).
		Delete(&models.RolesModuleAccess{}).Error; err != nil {
		return fmt.Errorf("delete DashboardPage permissions: %w", err)
	}
	if err := db.Unscoped().Delete(&models.AppModule{}, module.ID).Error; err != nil {
		return fmt.Errorf("delete DashboardPage module: %w", err)
	}
	return nil
}

// PruneStaleRoleScopes rewrites saved permission rows whose DataScope the
// backend cannot honour — a company-level module left at Own or Department by
// the old UI, which silently behaved as Companies anyway. This only makes the
// stored value agree with the behaviour that was already in effect; no role
// gains or loses access.
func PruneStaleRoleScopes(db *gorm.DB) error {
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

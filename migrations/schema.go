package migrations

import (
	"fmt"
	"log"
	"payroll-backend/models"

	"gorm.io/gorm"
)

func ensureSchemaMigrationTable(db *gorm.DB) error {
	return db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migration (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error
}

func RunOnce(db *gorm.DB, name string, migration func(*gorm.DB) error) error {
	if err := ensureSchemaMigrationTable(db); err != nil {
		return fmt.Errorf("ensure schema_migration table: %w", err)
	}

	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM schema_migration WHERE name = ?", name).Scan(&count).Error; err != nil {
		return fmt.Errorf("check schema migration %s: %w", name, err)
	}
	if count > 0 {
		log.Printf("Skipping migration %s (already applied)", name)
		return nil
	}

	log.Printf("Running migration %s", name)
	if err := migration(db); err != nil {
		return fmt.Errorf("run schema migration %s: %w", name, err)
	}
	if err := db.Exec("INSERT INTO schema_migration (name) VALUES (?) ON CONFLICT (name) DO NOTHING", name).Error; err != nil {
		return fmt.Errorf("mark schema migration %s applied: %w", name, err)
	}

	return nil
}

func DropLegacyIndexes(db *gorm.DB) error {
	legacyIndexes := []string{
		"idx_emp_type_name_deleted",
	}

	for _, indexName := range legacyIndexes {
		if db.Migrator().HasIndex(&models.EmploymentType{}, indexName) {
			if err := db.Migrator().DropIndex(&models.EmploymentType{}, indexName); err != nil {
				return err
			}
		}
	}

	return nil
}

// RecreateTaxIDPartialIndex replaces the unconditional unique index on tax_id with a partial one.
func RecreateTaxIDPartialIndex(db *gorm.DB) error {
	if db.Migrator().HasIndex(&models.Companies{}, "idx_taxid_deleted") {
		return db.Migrator().DropIndex(&models.Companies{}, "idx_taxid_deleted")
	}
	return nil
}

func RenameEmployeeCompanyAccessTable(db *gorm.DB) error {
	if db.Migrator().HasTable("employee_company_role") && !db.Migrator().HasTable("employee_company_access") {
		return db.Migrator().RenameTable("employee_company_role", "employee_company_access")
	}
	return nil
}

func BackfillSalariesStructureCompanies(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.SalariesStructure{}) {
		return nil
	}
	if !db.Migrator().HasTable(&models.Employees{}) || !db.Migrator().HasTable(&models.Companies{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&models.SalariesStructure{}, "companies_id") {
		if err := db.Migrator().AddColumn(&models.SalariesStructure{}, "CompaniesID"); err != nil {
			return err
		}
	}
	return db.Exec(`
		UPDATE salaries_structure AS ss
		SET companies_id = e.companies_id
		FROM employees AS e
		WHERE e.id = ss.employees_id
			AND EXISTS (SELECT 1 FROM companies AS c WHERE c.id = e.companies_id)
			AND (
				ss.companies_id = 0
				OR NOT EXISTS (
					SELECT 1
					FROM companies AS existing_company
					WHERE existing_company.id = ss.companies_id
				)
			)
	`).Error
}

func RemoveLegacyElevatedRole(db *gorm.DB) error {
	const legacyRoleName = "SUPER_ADMIN"
	const systemRoleName = "SYSTEM_ADMIN"

	if !db.Migrator().HasTable(&models.Roles{}) {
		return nil
	}

	// Rename legacy seeded roles when no replacement exists for that company.
	if err := db.Exec(`
		UPDATE roles AS legacy
		SET roles_name = ?
		WHERE legacy.roles_name = ?
			AND legacy.deleted_at = 0
			AND NOT EXISTS (
				SELECT 1
				FROM roles AS replacement
				WHERE replacement.companies_id = legacy.companies_id
					AND replacement.roles_name = ?
					AND replacement.deleted_at = 0
			)
	`, systemRoleName, legacyRoleName, systemRoleName).Error; err != nil {
		return err
	}

	duplicateCleanup := []struct {
		table string
		query string
	}{
		{
			table: "employee_company_access",
			query: `DELETE FROM employee_company_access AS ecr
				USING roles AS legacy, roles AS replacement, employee_company_access AS existing
				WHERE legacy.roles_name = 'SUPER_ADMIN'
					AND legacy.deleted_at = 0
					AND legacy.id = ecr.role_id
					AND replacement.companies_id = legacy.companies_id
					AND replacement.roles_name = 'SYSTEM_ADMIN'
					AND replacement.deleted_at = 0
					AND existing.employee_id = ecr.employee_id
					AND existing.company_id = ecr.company_id
					AND existing.role_id = replacement.id
					AND existing.deleted_at = 0`,
		},
		{
			table: "roles_module_access",
			query: `DELETE FROM roles_module_access AS rma
				USING roles AS legacy, roles AS replacement, roles_module_access AS existing
				WHERE legacy.roles_name = 'SUPER_ADMIN'
					AND legacy.deleted_at = 0
					AND legacy.id = rma.roles_id
					AND replacement.companies_id = legacy.companies_id
					AND replacement.roles_name = 'SYSTEM_ADMIN'
					AND replacement.deleted_at = 0
					AND existing.roles_id = replacement.id
					AND existing.module_id = rma.module_id
					AND existing.deleted_at = 0`,
		},
	}
	for _, cleanup := range duplicateCleanup {
		if !db.Migrator().HasTable(cleanup.table) {
			continue
		}
		if err := db.Exec(cleanup.query).Error; err != nil {
			return err
		}
	}

	// If both names exist, move references to the replacement then deactivate the legacy row.
	referenceUpdates := []struct {
		table string
		query string
	}{
		{
			table: "employees",
			query: `UPDATE employees AS e
			 SET roles_id = replacement.id
			 FROM roles AS legacy, roles AS replacement
			 WHERE legacy.roles_name = 'SUPER_ADMIN'
				AND legacy.deleted_at = 0
				AND legacy.id = e.roles_id
				AND replacement.companies_id = legacy.companies_id
				AND replacement.roles_name = 'SYSTEM_ADMIN'
				AND replacement.deleted_at = 0`,
		},
		{
			table: "employee_company_access",
			query: `UPDATE employee_company_access AS ecr
			 SET role_id = replacement.id
			 FROM roles AS legacy, roles AS replacement
			 WHERE legacy.roles_name = 'SUPER_ADMIN'
				AND legacy.deleted_at = 0
				AND legacy.id = ecr.role_id
				AND replacement.companies_id = legacy.companies_id
				AND replacement.roles_name = 'SYSTEM_ADMIN'
				AND replacement.deleted_at = 0`,
		},
		{
			table: "roles_module_access",
			query: `UPDATE roles_module_access AS rma
			 SET roles_id = replacement.id
			 FROM roles AS legacy, roles AS replacement
			 WHERE legacy.roles_name = 'SUPER_ADMIN'
				AND legacy.deleted_at = 0
				AND legacy.id = rma.roles_id
				AND replacement.companies_id = legacy.companies_id
				AND replacement.roles_name = 'SYSTEM_ADMIN'
				AND replacement.deleted_at = 0`,
		},
	}
	for _, update := range referenceUpdates {
		if !db.Migrator().HasTable(update.table) {
			continue
		}
		if err := db.Exec(update.query).Error; err != nil {
			return err
		}
	}
	if err := db.Exec("UPDATE roles SET deleted_at = CAST(EXTRACT(EPOCH FROM NOW()) AS bigint) WHERE roles_name = ? AND deleted_at = 0", legacyRoleName).Error; err != nil {
		return err
	}

	columnTypes, err := db.Migrator().ColumnTypes("roles")
	if err != nil {
		return err
	}
	for _, columnType := range columnTypes {
		if columnType.Name() == "is_super_admin" {
			return db.Exec("ALTER TABLE roles DROP COLUMN is_super_admin").Error
		}
	}

	return nil
}

// NormalizeSalaryActiveFlags rewrites `is_active` from its old meaning to its current one.
//
// It used to mark the single record in force, so every superseded record was switched off. It now marks a record
// as switched on — which record is in force is read from the effective dates — so those superseded records must
// come back on, or a run would price the periods they cover as if the employee had no salary then.
//
// An employee whose records are *all* off was deliberately disabled, and stays that way: the rewrite only touches
// employees who still have something switched on.
func NormalizeSalaryActiveFlags(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.SalariesStructure{}) {
		return nil
	}

	result := db.Exec(`
		UPDATE salaries_structure AS target
		SET is_active = true
		WHERE target.deleted_at = 0
		  AND target.is_active = false
		  AND EXISTS (
			SELECT 1 FROM salaries_structure AS live
			WHERE live.employees_id = target.employees_id
			  AND live.deleted_at = 0
			  AND live.is_active = true
		  )
	`)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("Switched %d superseded salary record(s) back on for the timeline-based lookup", result.RowsAffected)
	}
	return nil
}

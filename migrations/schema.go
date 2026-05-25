package migrations

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

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

func RenameEmployeeCompanyAccessTable(db *gorm.DB) error {
	if db.Migrator().HasTable("employee_company_role") && !db.Migrator().HasTable("employee_company_access") {
		return db.Migrator().RenameTable("employee_company_role", "employee_company_access")
	}
	return nil
}

func BackfillEmployeeCompanyAccess(db *gorm.DB) error {
	var employees []models.Employees
	if err := db.Unscoped().
		Select("id, companies_id, roles_id").
		Where("companies_id <> 0 AND roles_id <> 0").
		Find(&employees).Error; err != nil {
		return err
	}

	for _, employee := range employees {
		mapping := models.EmployeeCompanyAccess{
			EmployeeID: employee.ID,
			CompanyID:  employee.CompaniesID,
			RoleID:     employee.RolesID,
			IsDefault:  true,
			IsActive:   true,
		}
		if err := db.Where(models.EmployeeCompanyAccess{
			EmployeeID: employee.ID,
			CompanyID:  employee.CompaniesID,
			RoleID:     employee.RolesID,
		}).Assign(mapping).FirstOrCreate(&mapping).Error; err != nil {
			return err
		}
	}

	return nil
}

func BackfillSalariesStructureCompanies(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.SalariesStructure{}) {
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

	duplicateCleanup := []string{
		`DELETE FROM employee_company_access AS ecr
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
		`DELETE FROM roles_module_access AS rma
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
	}
	for _, query := range duplicateCleanup {
		if err := db.Exec(query).Error; err != nil {
			return err
		}
	}

	// If both names exist, move references to the replacement then deactivate the legacy row.
	referenceUpdates := []string{
		`UPDATE employees AS e
		 SET roles_id = replacement.id
		 FROM roles AS legacy, roles AS replacement
		 WHERE legacy.roles_name = 'SUPER_ADMIN'
			AND legacy.deleted_at = 0
			AND legacy.id = e.roles_id
			AND replacement.companies_id = legacy.companies_id
			AND replacement.roles_name = 'SYSTEM_ADMIN'
			AND replacement.deleted_at = 0`,
		`UPDATE employee_company_access AS ecr
		 SET role_id = replacement.id
		 FROM roles AS legacy, roles AS replacement
		 WHERE legacy.roles_name = 'SUPER_ADMIN'
			AND legacy.deleted_at = 0
			AND legacy.id = ecr.role_id
			AND replacement.companies_id = legacy.companies_id
			AND replacement.roles_name = 'SYSTEM_ADMIN'
			AND replacement.deleted_at = 0`,
		`UPDATE roles_module_access AS rma
		 SET roles_id = replacement.id
		 FROM roles AS legacy, roles AS replacement
		 WHERE legacy.roles_name = 'SUPER_ADMIN'
			AND legacy.deleted_at = 0
			AND legacy.id = rma.roles_id
			AND replacement.companies_id = legacy.companies_id
			AND replacement.roles_name = 'SYSTEM_ADMIN'
			AND replacement.deleted_at = 0`,
	}
	for _, query := range referenceUpdates {
		if err := db.Exec(query).Error; err != nil {
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

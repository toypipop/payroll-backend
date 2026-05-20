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
	if !db.Migrator().HasColumn(&models.SalariesStructure{}, "companies_id") {
		return nil
	}
	return db.Exec(`
		UPDATE salaries_structure ss
		JOIN employees e ON e.id = ss.employees_id
		SET ss.companies_id = e.companies_id
		WHERE ss.companies_id = 0
	`).Error
}

func RemoveLegacyElevatedRole(db *gorm.DB) error {
	const legacyRoleName = "SUPER_ADMIN"
	const systemRoleName = "SYSTEM_ADMIN"

	// Rename legacy seeded roles when no replacement exists for that company.
	if err := db.Exec(`
		UPDATE roles legacy
		LEFT JOIN roles replacement
			ON replacement.companies_id = legacy.companies_id
			AND replacement.roles_name = ?
			AND replacement.deleted_at = 0
		SET legacy.roles_name = ?
		WHERE legacy.roles_name = ?
			AND legacy.deleted_at = 0
			AND replacement.id IS NULL
	`, systemRoleName, systemRoleName, legacyRoleName).Error; err != nil {
		return err
	}

	duplicateCleanup := []string{
		`DELETE ecr
		 FROM employee_company_access ecr
		 JOIN roles legacy ON legacy.id = ecr.role_id
		 JOIN roles replacement ON replacement.companies_id = legacy.companies_id
			AND replacement.roles_name = 'SYSTEM_ADMIN'
			AND replacement.deleted_at = 0
		 JOIN employee_company_access existing ON existing.employee_id = ecr.employee_id
			AND existing.company_id = ecr.company_id
			AND existing.role_id = replacement.id
			AND existing.deleted_at = 0
		 WHERE legacy.roles_name = 'SUPER_ADMIN'
			AND legacy.deleted_at = 0`,
		`DELETE rma
		 FROM roles_module_access rma
		 JOIN roles legacy ON legacy.id = rma.roles_id
		 JOIN roles replacement ON replacement.companies_id = legacy.companies_id
			AND replacement.roles_name = 'SYSTEM_ADMIN'
			AND replacement.deleted_at = 0
		 JOIN roles_module_access existing ON existing.roles_id = replacement.id
			AND existing.module_id = rma.module_id
			AND existing.deleted_at = 0
		 WHERE legacy.roles_name = 'SUPER_ADMIN'
			AND legacy.deleted_at = 0`,
	}
	for _, query := range duplicateCleanup {
		if err := db.Exec(query).Error; err != nil {
			return err
		}
	}

	// If both names exist, move references to the replacement then deactivate the legacy row.
	referenceUpdates := []string{
		`UPDATE employees e
		 JOIN roles legacy ON legacy.id = e.roles_id
		 JOIN roles replacement ON replacement.companies_id = legacy.companies_id
			AND replacement.roles_name = 'SYSTEM_ADMIN'
			AND replacement.deleted_at = 0
		 SET e.roles_id = replacement.id
		 WHERE legacy.roles_name = 'SUPER_ADMIN'
			AND legacy.deleted_at = 0`,
		`UPDATE employee_company_access ecr
		 JOIN roles legacy ON legacy.id = ecr.role_id
		 JOIN roles replacement ON replacement.companies_id = legacy.companies_id
			AND replacement.roles_name = 'SYSTEM_ADMIN'
			AND replacement.deleted_at = 0
		 SET ecr.role_id = replacement.id
		 WHERE legacy.roles_name = 'SUPER_ADMIN'
			AND legacy.deleted_at = 0`,
		`UPDATE roles_module_access rma
		 JOIN roles legacy ON legacy.id = rma.roles_id
		 JOIN roles replacement ON replacement.companies_id = legacy.companies_id
			AND replacement.roles_name = 'SYSTEM_ADMIN'
			AND replacement.deleted_at = 0
		 SET rma.roles_id = replacement.id
		 WHERE legacy.roles_name = 'SUPER_ADMIN'
			AND legacy.deleted_at = 0`,
	}
	for _, query := range referenceUpdates {
		if err := db.Exec(query).Error; err != nil {
			return err
		}
	}
	if err := db.Exec("UPDATE roles SET deleted_at = UNIX_TIMESTAMP() WHERE roles_name = ? AND deleted_at = 0", legacyRoleName).Error; err != nil {
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

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

func BackfillEmployeeCompanyRoles(db *gorm.DB) error {
	var employees []models.Employees
	if err := db.Unscoped().
		Select("id, companies_id, roles_id").
		Where("companies_id <> 0 AND roles_id <> 0").
		Find(&employees).Error; err != nil {
		return err
	}

	for _, employee := range employees {
		mapping := models.EmployeeCompanyRole{
			EmployeeID: employee.ID,
			CompanyID:  employee.CompaniesID,
			RoleID:     employee.RolesID,
			IsDefault:  true,
			IsActive:   true,
		}
		if err := db.Where(models.EmployeeCompanyRole{
			EmployeeID: employee.ID,
			CompanyID:  employee.CompaniesID,
			RoleID:     employee.RolesID,
		}).Assign(mapping).FirstOrCreate(&mapping).Error; err != nil {
			return err
		}
	}

	return nil
}

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

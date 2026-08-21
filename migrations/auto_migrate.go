package migrations

import (
	"log"

	"payroll-backend/models"

	"gorm.io/gorm"
)

func AutoMigrateSchema(db *gorm.DB) error {
	// Must run before AutoMigrate: the unique index on payroll_transaction cannot be created while duplicate rows exist.
	if err := DedupePayrollTransactions(db); err != nil {
		return err
	}

	return db.AutoMigrate(
		&models.Companies{},
		&models.Roles{},
		&models.AppModule{},
		&models.RolesModuleAccess{},
		&models.DepartmentCategory{},
		&models.Department{},
		&models.Position{},
		&models.Employees{},
		&models.SalariesStructure{},
		&models.PayrollCycle{},
		&models.PayrollTransaction{},
		&models.PayrollTransactionDetail{},
		&models.TimeLog{},
		&models.LeaveRequest{},
		&models.LeaveBalance{},
		&models.PayrollConfig{},
		&models.EarningType{},
		&models.DeductionType{},
		&models.EmployeeEarning{},
		&models.EmployeeDeduction{},
		&models.PayrollCycleEarning{},
		&models.PayrollCycleDeduction{},
		&models.AttendanceConfig{},
		&models.Bank{},
		&models.EmploymentType{},
		&models.Holiday{},
		&models.DayOff{},
		&models.WorkLocation{},
		&models.LeaveTypeSetting{},
		&models.EmployeeCompanyAccess{},
	)
}

// DedupePayrollTransactions keeps the most recently updated live transaction per (cycle_id, employees_id) and erases the rest with their detail lines.
// Find-then-create used to be the only thing stopping two concurrent Calculate requests from both inserting, so an existing database can already hold duplicates that idx_txn_cycle_employee would reject.
func DedupePayrollTransactions(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.PayrollTransaction{}) {
		return nil
	}

	const duplicates = `
		SELECT id FROM (
			SELECT id, ROW_NUMBER() OVER (
				PARTITION BY cycle_id, employees_id
				ORDER BY updated_at DESC, id DESC
			) AS rn
			FROM payroll_transaction
			WHERE deleted_at = 0
		) ranked WHERE rn > 1
	`

	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&models.PayrollTransactionDetail{}) {
			if err := tx.Exec(`DELETE FROM payroll_transaction_detail WHERE transaction_id IN (` + duplicates + `)`).Error; err != nil {
				return err
			}
		}
		result := tx.Exec(`DELETE FROM payroll_transaction WHERE id IN (` + duplicates + `)`)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			log.Printf("Removed %d duplicate payroll transaction(s) before creating idx_txn_cycle_employee", result.RowsAffected)
		}
		return nil
	})
}

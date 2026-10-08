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

	if err := autoMigrateTables(db); err != nil {
		return err
	}
	if err := ensureAuditLogIndexes(db); err != nil {
		return err
	}
	if err := BackfillSalaryAudits(db); err != nil {
		return err
	}

	// A row added before the optimistic lock existed would carry version 0, and every write guarded by
	// `version = <what was read>` would then compare against a value the client never saw.
	if err := db.Exec(`UPDATE salaries_structure SET version = 1 WHERE version = 0`).Error; err != nil {
		return err
	}

	// Erasing a salary record used to set `deleted_at` and leave `is_active = true`. Those rows still read as
	// current to anything that lists records unscoped, while soft delete keeps every UPDATE away from them.
	return db.Exec(`UPDATE salaries_structure SET is_active = false WHERE deleted_at <> 0 AND is_active = true`).Error
}

func autoMigrateTables(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Companies{},
		&models.Roles{},
		&models.AppModule{},
		&models.RolesModuleAccess{},
		&models.DepartmentCategory{},
		&models.Department{},
		&models.Position{},
		&models.Employees{},
		&models.AuthSession{},
		&models.SalariesStructure{},
		&models.SalaryAudit{},
		&models.AuditLog{},
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

func ensureAuditLogIndexes(db *gorm.DB) error {
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_audit_log_company_occurred ON audit_log (companies_id, occurred_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_entity_occurred ON audit_log (entity_type, entity_id, occurred_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_actor_occurred ON audit_log (actor_user_id, occurred_at DESC)`,
	}
	for _, statement := range indexes {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
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

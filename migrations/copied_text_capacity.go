package migrations

import "gorm.io/gorm"

// Only widen bounded columns. Never narrow an unbounded text column or cast
// with truncation. Running again is harmless and all changes share a transaction.
func WidenCopiedTextColumns(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, column := range []struct {
			table, name, statement string
			width                  int
		}{
			{"leave_request", "leave_type", `ALTER TABLE leave_request ALTER COLUMN leave_type TYPE varchar(100)`, 100},
			{"leave_balance", "leave_type", `ALTER TABLE leave_balance ALTER COLUMN leave_type TYPE varchar(100)`, 100},
			{"payroll_transaction", "bank_name", `ALTER TABLE payroll_transaction ALTER COLUMN bank_name TYPE varchar(200)`, 200},
		} {
			var existing struct {
				Maximum *int `gorm:"column:character_maximum_length"`
			}
			if err := tx.Raw(`SELECT character_maximum_length FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`, column.table, column.name).Scan(&existing).Error; err != nil {
				return err
			}
			if existing.Maximum != nil && *existing.Maximum < column.width {
				if err := tx.Exec(column.statement).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

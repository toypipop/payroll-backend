package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// SalariesStructure defines the base pay for each employee
type SalariesStructure struct {
	// Standard Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Composite Primary Key often doesn't use soft delete,
	// but we add it to track salaries history/changes (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_salaries_deleted;not null;default:0" json:"-"`

	EmployeeID    uint      `gorm:"primaryKey;uniqueIndex:idx_emp_salaries_deleted" json:"employee_id"`
	BaseSalaries  float64   `gorm:"type:decimal(15,2)" json:"base_salaries"`
	EffectiveDate time.Time `json:"effective_date"`
}

// PayrollCycle manages the time periods for salaries processing
type PayrollCycle struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Allow re-creating same cycle name after deletion (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_cycle_name_comp_deleted;not null;default:0" json:"-"`

	CompaniesID uint `gorm:"uniqueIndex:idx_cycle_name_comp_deleted;not null" json:"companies_id"`
	// Added size:255 for Index support (English Comment)
	CycleName string    `gorm:"uniqueIndex:idx_cycle_name_comp_deleted;not null;size:255" json:"cycle_name"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	IsClosed  bool      `gorm:"default:false" json:"is_closed"`
}

// PayrollTransaction records the actual payout for an employee
type PayrollTransaction struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"` // Date of transaction
	UpdatedAt time.Time `json:"updated_at"`
	// Transactions usually use soft delete for audit purposes (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"index" json:"-"`

	CycleID        uint    `gorm:"not null" json:"cycle_id"`
	EmployeeID     uint    `gorm:"not null" json:"employee_id"`
	SalariesAtTime float64 `gorm:"type:decimal(15,2)" json:"salaries_at_time"`
	NetPay         float64 `gorm:"type:decimal(15,2)" json:"net_pay"`
}

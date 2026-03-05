package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Employees struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Use Composite Unique Index with DeletedAt for both EmployeesCode and Email
	// Set default:0 to ensure unique index works correctly with soft delete (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_code_companies_deleted;uniqueIndex:idx_email_companies_deleted;not null;default:0" json:"-"`

	// Multi-tenancy support
	CompaniesID uint `gorm:"uniqueIndex:idx_emp_code_companies_deleted;uniqueIndex:idx_email_companies_deleted;not null" json:"companies_id"`

	// Added size:100 to prevent BLOB/TEXT error in MySQL Index (English Comment)
	EmployeesCode string `gorm:"uniqueIndex:idx_emp_code_companies_deleted;not null;size:100" json:"employees_code"`
	Email         string `gorm:"uniqueIndex:idx_email_companies_deleted;not null;size:150" json:"email"`

	FirstName string `gorm:"not null" json:"first_name"`
	LastName  string `gorm:"not null" json:"last_name"`
	Password  string `gorm:"not null" json:"password,omitempty"`

	DeptID  uint `json:"dept_id"`
	PosID   uint `json:"pos_id"`
	RolesID uint `gorm:"not null" json:"roles_id"`

	IsActive bool `gorm:"default:true" json:"is_active"`

	// Relations
	Department        Department        `gorm:"foreignKey:DeptID" json:"department"`
	Position          Position          `gorm:"foreignKey:PosID" json:"position"`
	Roles             Roles             `gorm:"foreignKey:RolesID" json:"roles"`
	SalariesStructure SalariesStructure `gorm:"foreignKey:EmployeesID" json:"salaries_structure"`
}

// SalariesStructure defines the base pay for each Employees
type SalariesStructure struct {
	// Standard Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Composite Primary Key often doesn't use soft delete,
	// but we add it to track salaries history/changes (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_salaries_deleted;not null;default:0" json:"-"`

	EmployeesID   uint      `gorm:"primaryKey;uniqueIndex:idx_emp_salaries_deleted" json:"Employees_id"`
	BaseSalaries  float64   `gorm:"type:decimal(15,2)" json:"base_salaries"`
	EffectiveDate time.Time `json:"effective_date"`
	IsEnabled     bool      `json:"is_enabled" gorm:"default:true"`
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

// PayrollTransaction records the actual payout for an Employees
type PayrollTransaction struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"` // Date of transaction
	UpdatedAt time.Time `json:"updated_at"`
	// Transactions usually use soft delete for audit purposes (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"index" json:"-"`

	CycleID        uint    `gorm:"not null" json:"cycle_id"`
	EmployeesID    uint    `gorm:"not null" json:"Employees_id"`
	SalariesAtTime float64 `gorm:"type:decimal(15,2)" json:"salaries_at_time"`
	NetPay         float64 `gorm:"type:decimal(15,2)" json:"net_pay"`
}

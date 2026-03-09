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
	ID            uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	DeletedAt     soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_sal_eff_deleted;not null;default:0" json:"-"`
	EmployeesID   uint                  `gorm:"not null;index;uniqueIndex:idx_emp_sal_eff_deleted" json:"employees_id"`
	BaseSalaries  float64               `gorm:"type:decimal(15,2)" json:"base_salaries"`
	EffectiveDate time.Time             `gorm:"not null;uniqueIndex:idx_emp_sal_eff_deleted" json:"effective_date"`
	IsEnabled     bool                  `json:"is_enabled" gorm:"default:true"`
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
	ID             uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
	DeletedAt      soft_delete.DeletedAt `gorm:"index" json:"-"`
	CycleID        uint                  `gorm:"not null" json:"cycle_id"`
	EmployeesID    uint                  `gorm:"not null" json:"employees_id"`
	SalariesAtTime float64               `gorm:"type:decimal(15,2)" json:"salaries_at_time"`
	TotalEarning   float64               `gorm:"type:decimal(15,2)" json:"total_earning"`
	TotalDeduction float64               `gorm:"type:decimal(15,2)" json:"total_deduction"`
	SsoAmount      float64               `gorm:"type:decimal(15,2)" json:"sso_amount"`
	TaxAmount      float64               `gorm:"type:decimal(15,2)" json:"tax_amount"`
	NetPay         float64               `gorm:"type:decimal(15,2)" json:"net_pay"`
	// Relations (English Comment)
	Details   []PayrollTransactionDetail `gorm:"foreignKey:TransactionID" json:"details"`
	Employees Employees                  `gorm:"foreignKey:EmployeesID" json:"employees"`
}

type PayrollTransactionDetail struct {
	ID            uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time             `json:"created_at"`
	DeletedAt     soft_delete.DeletedAt `gorm:"index" json:"-"`
	TransactionID uint                  `gorm:"not null;index" json:"transaction_id"`
	ItemName      string                `gorm:"size:255" json:"item_name"` // e.g., "Overtime", "SSO", "Tax"
	Amount        float64               `gorm:"type:decimal(15,2)" json:"amount"`
	Type          string                `gorm:"size:20" json:"type"` // "earning" or "deduction"
	// Reference to master data if applicable (English Comment)
	EarningTypeID   *uint `json:"earning_type_id"`
	DeductionTypeID *uint `json:"deduction_type_id"`
}

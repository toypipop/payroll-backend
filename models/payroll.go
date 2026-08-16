package models

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

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

	Companies Companies `gorm:"foreignKey:CompaniesID" json:"companies"`
}

// PayrollTransaction records the actual payout for an Employees
type PayrollTransaction struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	DeletedAt   soft_delete.DeletedAt `gorm:"index" json:"-"`
	CycleID     uint                  `gorm:"not null" json:"cycle_id"`
	EmployeesID uint                  `gorm:"not null" json:"employees_id"`
	// Which salary record produced SalariesAtTime — lets the salary handler tell whether a superseded record was ever actually used in a real calculation.
	SalariesStructureID uint    `gorm:"not null;index" json:"salaries_structure_id"`
	SalariesAtTime      float64 `gorm:"type:decimal(15,2)" json:"salaries_at_time"`
	// Snapshot fields: captured at calculation time so historical payslips stay accurate
	PositionName       string  `gorm:"size:255" json:"position_name"`
	DepartmentName     string  `gorm:"size:255" json:"department_name"`
	EmploymentTypeName string  `gorm:"size:100" json:"employment_type_name"`
	BankName           string  `gorm:"size:100" json:"bank_name"`
	BankAccountName    string  `gorm:"size:200" json:"bank_account_name"`
	BankAccountNo      string  `gorm:"size:50" json:"bank_account_no"`
	TotalEarning       float64 `gorm:"type:decimal(15,2)" json:"total_earning"`
	TotalDeduction     float64 `gorm:"type:decimal(15,2)" json:"total_deduction"`
	SsoAmount          float64 `gorm:"type:decimal(15,2)" json:"sso_amount"`
	// TaxableIncome is income left after all tax-deductible items (SSO, provident fund, etc.) are subtracted — the base actually fed into the tax calculation.
	TaxableIncome float64 `gorm:"type:decimal(15,2)" json:"taxable_income"`
	TaxAmount     float64 `gorm:"type:decimal(15,2)" json:"tax_amount"`
	NetPay        float64 `gorm:"type:decimal(15,2)" json:"net_pay"`
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
	// IsTaxRelevant marks whether this line item actually fed into the tax
	IsTaxRelevant bool `json:"is_tax_relevant"`
	// Reference to master data if applicable (English Comment)
	EarningTypeID   *uint `json:"earning_type_id"`
	DeductionTypeID *uint `json:"deduction_type_id"`
}

// PayrollCycleEarning holds one-time variable earnings (is_permanent=false) for a specific payroll cycle
type PayrollCycleEarning struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	CompaniesID   uint           `gorm:"not null;index" json:"companies_id"`
	CycleID       uint           `gorm:"not null;index" json:"cycle_id"`
	EmployeesID   uint           `gorm:"not null;index" json:"employees_id"`
	EarningTypeID uint           `gorm:"not null" json:"earning_type_id"`
	Amount        float64        `gorm:"type:decimal(15,2);not null" json:"amount"`
	EarningType   EarningType    `gorm:"foreignKey:EarningTypeID" json:"earning_type"`
	Employees     Employees      `gorm:"foreignKey:EmployeesID" json:"employees"`
	PayrollCycle  PayrollCycle   `gorm:"foreignKey:CycleID" json:"payroll_cycle"`
}

// PayrollCycleDeduction holds one-time variable deductions for a specific payroll cycle.
type PayrollCycleDeduction struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	CompaniesID     uint           `gorm:"not null;index" json:"companies_id"`
	CycleID         uint           `gorm:"not null;index" json:"cycle_id"`
	EmployeesID     uint           `gorm:"not null;index" json:"employees_id"`
	DeductionTypeID uint           `gorm:"not null" json:"deduction_type_id"`
	Amount          float64        `gorm:"type:decimal(15,2);not null" json:"amount"`
	DeductionType   DeductionType  `gorm:"foreignKey:DeductionTypeID" json:"deduction_type"`
	Employees       Employees      `gorm:"foreignKey:EmployeesID" json:"employees"`
	PayrollCycle    PayrollCycle   `gorm:"foreignKey:CycleID" json:"payroll_cycle"`
}

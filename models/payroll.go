package models

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

type PayrollConfig struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	CompaniesID    uint           `gorm:"not null;index" json:"companies_id"`
	SsoRate        float64        `gorm:"type:decimal(5,2);default:5.0" json:"sso_rate"`
	SsoMaxAmount   float64        `gorm:"type:decimal(10,2);default:750" json:"sso_max_amount"`
	SsoMinSalary   float64        `gorm:"type:decimal(10,2);default:1650" json:"sso_min_salary"`
	TaxCalculation string         `gorm:"type:varchar(50);default:'Progressive'" json:"tax_calculation"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Companies      Companies      `gorm:"foreignKey:CompaniesID" json:"companies"`
}

type EarningType struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CompaniesID uint           `gorm:"not null;index" json:"companies_id"`
	Name        string         `gorm:"type:varchar(100);not null" json:"name"`
	IsTaxable   bool           `json:"is_taxable"`
	IsSso       bool           `json:"is_sso"`
	IsPermanent bool           `json:"is_permanent"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Companies   Companies      `gorm:"foreignKey:CompaniesID" json:"companies"`
}

type DeductionType struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	CompaniesID     uint           `gorm:"not null;index" json:"companies_id"`
	Name            string         `gorm:"type:varchar(100);not null" json:"name"`
	IsTaxDeductible bool           `json:"is_tax_deductible"`
	IsSsoDeductible bool           `json:"is_sso_deductible"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// EmployeeEarning assigns an additional earning type + fixed monthly amount to an employee
type EmployeeEarning struct {
	ID            uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	DeletedAt     soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_etype_deleted;not null;default:0" json:"-"`
	CompaniesID   uint                  `gorm:"not null;index" json:"companies_id"`
	EmployeesID   uint                  `gorm:"uniqueIndex:idx_emp_etype_deleted;not null;index" json:"employees_id"`
	EarningTypeID uint                  `gorm:"uniqueIndex:idx_emp_etype_deleted;not null" json:"earning_type_id"`
	Amount        float64               `gorm:"type:decimal(15,2);not null" json:"amount"`
	EarningType   EarningType           `gorm:"foreignKey:EarningTypeID" json:"earning_type"`
	Employees     Employees             `gorm:"foreignKey:EmployeesID" json:"employees"`
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

// EmployeeDeduction assigns a deduction type + fixed monthly amount to an employee
type EmployeeDeduction struct {
	ID              uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	DeletedAt       soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_dtype_deleted;not null;default:0" json:"-"`
	CompaniesID     uint                  `gorm:"not null;index" json:"companies_id"`
	EmployeesID     uint                  `gorm:"uniqueIndex:idx_emp_dtype_deleted;not null;index" json:"employees_id"`
	DeductionTypeID uint                  `gorm:"uniqueIndex:idx_emp_dtype_deleted;not null" json:"deduction_type_id"`
	Amount          float64               `gorm:"type:decimal(15,2);not null" json:"amount"`
	DeductionType   DeductionType         `gorm:"foreignKey:DeductionTypeID" json:"deduction_type"`
	Employees       Employees             `gorm:"foreignKey:EmployeesID" json:"employees"`
}

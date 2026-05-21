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
	ID          uint             `gorm:"primaryKey" json:"id"`
	CompaniesID uint             `gorm:"not null;index" json:"companies_id"`
	Name        string           `gorm:"type:varchar(100);not null" json:"name"`
	CategoryID  *uint            `json:"category_id"` // FK -> EarningCategory
	IsTaxable   bool             `json:"is_taxable"`
	IsSso       bool             `json:"is_sso"`
	IsPermanent bool             `json:"is_permanent"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	DeletedAt   gorm.DeletedAt   `gorm:"index" json:"-"`
	Companies   Companies        `gorm:"foreignKey:CompaniesID" json:"companies"`
	Category    *EarningCategory `gorm:"foreignKey:CategoryID" json:"category"`
}

type DeductionType struct {
	ID              uint               `gorm:"primaryKey" json:"id"`
	CompaniesID     uint               `gorm:"not null;index" json:"companies_id"`
	Name            string             `gorm:"type:varchar(100);not null" json:"name"`
	CategoryID      *uint              `json:"category_id"` // FK -> DeductionCategory
	IsTaxDeductible bool               `json:"is_tax_deductible"`
	IsSsoDeductible bool               `json:"is_sso_deductible"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	DeletedAt       gorm.DeletedAt     `gorm:"index" json:"-"`
	Companies       Companies          `gorm:"foreignKey:CompaniesID" json:"companies"`
	Category        *DeductionCategory `gorm:"foreignKey:CategoryID" json:"category"`
}

// EmploymentType is a per-company lookup for employment contract types
type EmploymentType struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_type_company_name_deleted;not null;default:0" json:"-"`

	CompaniesID uint       `gorm:"uniqueIndex:idx_emp_type_company_name_deleted;not null;default:0" json:"companies_id"`
	Companies   *Companies `gorm:"foreignKey:CompaniesID" json:"companies,omitempty"`
	Name        string     `gorm:"uniqueIndex:idx_emp_type_company_name_deleted;not null;size:100" json:"name"`
	Description string     `gorm:"size:255" json:"description"`
}

// EarningCategory groups earning types into logical categories (global)
type EarningCategory struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_ec_name_deleted;not null;default:0" json:"-"`

	Name string `gorm:"uniqueIndex:idx_ec_name_deleted;not null;size:100" json:"name"`
}

// DeductionCategory groups deduction types into logical categories (global)
type DeductionCategory struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_dc_name_deleted;not null;default:0" json:"-"`

	Name string `gorm:"uniqueIndex:idx_dc_name_deleted;not null;size:100" json:"name"`
}

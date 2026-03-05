package models

import (
	"time"

	"gorm.io/gorm"
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
	IsTaxable   bool           `gorm:"default:true" json:"is_taxable"`
	IsSso       bool           `gorm:"default:true" json:"is_sso"`
	IsPermanent bool           `gorm:"default:false" json:"is_permanent"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Companies   Companies      `gorm:"foreignKey:CompaniesID" json:"companies"`
}

type DeductionType struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CompaniesID uint           `gorm:"not null;index" json:"companies_id"`
	Name        string         `gorm:"type:varchar(100);not null" json:"name"`
	IsTaxDed    bool           `gorm:"default:false" json:"is_tax_ded"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Companies   Companies      `gorm:"foreignKey:CompaniesID" json:"companies"`
}

package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Department struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Use Composite Unique Index with DeletedAt to allow re-creating same dept name (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_dept_name_companies_deleted;not null;default:0" json:"-"`

	CompaniesID uint `gorm:"uniqueIndex:idx_dept_name_companies_deleted;not null" json:"companies_id"`
	// Add size:255 for MySQL index support (English Comment)
	DeptName string `gorm:"uniqueIndex:idx_dept_name_companies_deleted;not null;size:255" json:"dept_name"`

	Companies Companies `gorm:"foreignKey:CompaniesID" json:"companies"`
}

// Position represents job roles within a companies
type Position struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Use Composite Unique Index with DeletedAt to allow re-creating same position name (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_pos_name_companies_deleted;not null;default:0" json:"-"`

	CompaniesID uint `gorm:"uniqueIndex:idx_pos_name_companies_deleted;not null" json:"companies_id"`
	// Add size:255 for MySQL index support (English Comment)
	PosName string `gorm:"uniqueIndex:idx_pos_name_companies_deleted;not null;size:255" json:"pos_name"`

	Companies Companies `gorm:"foreignKey:CompaniesID" json:"companies"`
}

// Companies represents the tenant/entity information
type Companies struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_taxid_deleted;not null;default:0" json:"-"`
	Name      string                `gorm:"not null" json:"name"`
	TaxID     string                `gorm:"uniqueIndex:idx_taxid_deleted;not null;size:50" json:"tax_id"`
	Address   string                `json:"address"`
	Phone     string                `json:"phone"`
}

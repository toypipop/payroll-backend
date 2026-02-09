package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Department struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	CompaniesID uint   `gorm:"index;not null" json:"companies_id"` // Link to companies
	DeptName  string `gorm:"not null" json:"dept_name"`

	// Add this to enable Preload("Companies") (English Comment)
	Companies Companies `gorm:"foreignKey:CompaniesID" json:"companies"`
}

type Position struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	CompaniesID uint   `gorm:"index;not null" json:"companies_id"` // Link to companies
	PosName   string `gorm:"not null" json:"pos_name"`

	// Add this to enable Preload("Companies") (English Comment)
	Companies Companies `gorm:"foreignKey:CompaniesID" json:"companies"`
}

type Companies struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Use soft_delete.DeletedAt instead of gorm.DeletedAt (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"index:idx_taxid_deleted,unique" json:"-"`

	Name    string `json:"name" gorm:"not null"`
	TaxID   string `json:"tax_id" gorm:"index:idx_taxid_deleted,unique;not null"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
}

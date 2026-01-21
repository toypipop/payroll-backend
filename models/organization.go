package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Department struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	CompanyID uint   `gorm:"index;not null" json:"company_id"` // Link to company
	DeptName  string `gorm:"not null" json:"dept_name"`
}

type Position struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	CompanyID uint   `gorm:"index;not null" json:"company_id"` // Link to company
	PosName   string `gorm:"not null" json:"pos_name"`
}
type Company struct {
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

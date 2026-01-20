package models

import "gorm.io/gorm"

type Department struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	DeptName string `gorm:"not null" json:"dept_name"`
}

type Position struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	PosName string `gorm:"not null" json:"pos_name"`
}

type Company struct {
	gorm.Model
	/* --- Override DeletedAt to include it in the Composite Unique Index (English Comment) --- */
	DeletedAt gorm.DeletedAt `gorm:"index:idx_taxid_deleted,unique"`

	Name    string `json:"name" gorm:"not null"`
	TaxID   string `json:"tax_id" gorm:"index:idx_taxid_deleted,unique;not null"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
}

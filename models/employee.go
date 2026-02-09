package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Employee struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Add CompaniesID to support Multi-tenancy (English Comment)
	CompaniesID uint                  `gorm:"index;not null" json:"companies_id"`
	DeletedAt soft_delete.DeletedAt `gorm:"index:idx_emp_code_companies_deleted,unique;index:idx_email_companies_deleted,unique" json:"-"`

	// Update index to include CompaniesID (English Comment)
	EmployeeCode string `gorm:"index:idx_emp_code_companies_deleted,unique;not null" json:"employee_code"`
	Email        string `gorm:"index:idx_email_companies_deleted,unique" json:"email"`

	FirstName string `gorm:"not null" json:"first_name"`
	LastName  string `gorm:"not null" json:"last_name"`
	Password  string `gorm:"not null" json:"password,omitempty"`
	DeptID    uint   `json:"dept_id"`
	PosID     uint   `json:"pos_id"`
	RoleID    uint   `gorm:"not null" json:"role_id"`

	IsActive bool `gorm:"default:true" json:"is_active"`

	Department Department `gorm:"foreignKey:DeptID" json:"department"`
	Position   Position   `gorm:"foreignKey:PosID" json:"position"`
	Role       Role       `gorm:"foreignKey:RoleID" json:"role"`
}

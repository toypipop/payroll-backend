package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Employee struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Use Composite Unique Index with DeletedAt for both EmployeeCode and Email
	// Set default:0 to ensure unique index works correctly with soft delete (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_code_companies_deleted;uniqueIndex:idx_email_companies_deleted;not null;default:0" json:"-"`

	// Multi-tenancy support
	CompaniesID uint `gorm:"uniqueIndex:idx_emp_code_companies_deleted;uniqueIndex:idx_email_companies_deleted;not null" json:"companies_id"`

	// Added size:100 to prevent BLOB/TEXT error in MySQL Index (English Comment)
	EmployeeCode string `gorm:"uniqueIndex:idx_emp_code_companies_deleted;not null;size:100" json:"employee_code"`
	Email        string `gorm:"uniqueIndex:idx_email_companies_deleted;not null;size:150" json:"email"`

	FirstName string `gorm:"not null" json:"first_name"`
	LastName  string `gorm:"not null" json:"last_name"`
	Password  string `gorm:"not null" json:"password,omitempty"`

	DeptID uint `json:"dept_id"`
	PosID  uint `json:"pos_id"`
	RoleID uint `gorm:"not null" json:"role_id"`

	IsActive bool `gorm:"default:true" json:"is_active"`

	// Relations
	Department Department `gorm:"foreignKey:DeptID" json:"department"`
	Position   Position   `gorm:"foreignKey:PosID" json:"position"`
	Role       Role       `gorm:"foreignKey:RoleID" json:"role"`
}

package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

type Employee struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Add CompanyID to support Multi-tenancy (English Comment)
	CompanyID uint                  `gorm:"index;not null" json:"company_id"`
	DeletedAt soft_delete.DeletedAt `gorm:"index:idx_emp_code_company_deleted,unique;index:idx_email_company_deleted,unique" json:"-"`

	// Update index to include CompanyID (English Comment)
	EmployeeCode string `gorm:"index:idx_emp_code_company_deleted,unique;not null" json:"employee_code"`
	Email        string `gorm:"index:idx_email_company_deleted,unique" json:"email"`

	FirstName string `gorm:"not null" json:"first_name"`
	LastName  string `gorm:"not null" json:"last_name"`
	Password  string `gorm:"not null" json:"-"`
	DeptID    uint   `json:"dept_id"`
	PosID     uint   `json:"pos_id"`
	Status    string `gorm:"default:'ACTIVE'" json:"status"`

	Department Department `gorm:"foreignKey:DeptID" json:"department"`
	Position   Position   `gorm:"foreignKey:PosID" json:"position"`
}

// BeforeCreate hook for password hashing (English Comment)
func (e *Employee) BeforeCreate(tx *gorm.DB) (err error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(e.Password), 10)
	if err != nil {
		return err
	}
	e.Password = string(hashedPassword)
	return nil
}

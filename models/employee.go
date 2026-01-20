package models

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Employee struct {
	gorm.Model
	/* --- Override DeletedAt to include it in Composite Unique Index (English Comment) --- */
	// Add the same index name to DeletedAt to create a (Code + DeletedAt) unique pair
	DeletedAt gorm.DeletedAt `gorm:"index:idx_emp_code_deleted,unique;index:idx_email_deleted,unique"`

	EmployeeCode string `gorm:"index:idx_emp_code_deleted,unique;not null" json:"employee_code"`
	Email        string `gorm:"index:idx_email_deleted,unique" json:"email"`

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

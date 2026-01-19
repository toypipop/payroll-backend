package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// --- System & Authorization ---

type Role struct {
	ID          uint   `gorm:"primaryKey"`
	RoleName    string `gorm:"unique;not null"`
	Description string
}

type AppModule struct {
	ID         uint   `gorm:"primaryKey"`
	ModuleName string `gorm:"not null"`
	ModuleKey  string `gorm:"unique;not null"`
	RoutePath  string
}

type RoleModuleAccess struct {
	RoleID    uint   `gorm:"primaryKey"`
	ModuleID  uint   `gorm:"primaryKey"`
	CanView   bool   `gorm:"default:true"`
	CanEdit   bool   `gorm:"default:false"`
	DataScope string `gorm:"type:enum('ALL','DEPARTMENT','OWN');default:'OWN'"`
}

// --- Organization & Employee ---

type Department struct {
	ID       uint   `gorm:"primaryKey"`
	DeptName string `gorm:"not null"`
}

type Position struct {
	ID      uint   `gorm:"primaryKey"`
	PosName string `gorm:"not null"`
}

type Employee struct {
	ID           uint   `gorm:"primaryKey"`
	EmployeeCode string `gorm:"unique;not null"`
	FirstName    string `gorm:"not null"`
	LastName     string `gorm:"not null"`
	Email        string `gorm:"unique"`
	Password     string `gorm:"not null"`
	DeptID       uint
	PosID        uint
	Status       string `gorm:"default:'ACTIVE'"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// Relationships
	Department Department `gorm:"foreignKey:DeptID"`
	Position   Position   `gorm:"foreignKey:PosID"`
}

// BeforeCreate is a GORM hook that runs before inserting a new record
func (e *Employee) BeforeCreate(tx *gorm.DB) (err error) {
	// Hash password with a cost of 10
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(e.Password), 10)
	if err != nil {
		return err
	}

	// Replace plain text password with hashed password
	e.Password = string(hashedPassword)
	return nil
}

// --- Payroll Processing ---

type SalaryStructure struct {
	EmployeeID uint    `gorm:"primaryKey"`
	BaseSalary float64 `gorm:"type:decimal(15,2)"`
}

type PayrollCycle struct {
	ID        uint `gorm:"primaryKey"`
	CycleName string
	StartDate time.Time
	EndDate   time.Time
	IsClosed  bool `gorm:"default:false"`
}

type PayrollTransaction struct {
	ID           uint `gorm:"primaryKey"`
	CycleID      uint
	EmployeeID   uint
	SalaryAtTime float64 `gorm:"type:decimal(15,2)"`
	NetPay       float64 `gorm:"type:decimal(15,2)"`
	// Snapshot data
	CreatedAt time.Time
}

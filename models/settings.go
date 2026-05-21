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
	ID               uint                    `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
	DeletedAt        soft_delete.DeletedAt   `gorm:"uniqueIndex:idx_taxid_deleted;not null;default:0" json:"-"`
	Name             string                  `gorm:"not null" json:"name"`
	TaxID            string                  `gorm:"uniqueIndex:idx_taxid_deleted;not null;size:50" json:"tax_id"`
	Address          string                  `json:"address"`
	Phone            string                  `json:"phone"`
	PayrollConfig    *PayrollConfig          `gorm:"foreignKey:CompaniesID" json:"payroll_config"`
	AttendanceConfig *AttendanceConfig       `gorm:"foreignKey:CompaniesID" json:"attendance_config"`
	EmployeeAccess   []EmployeeCompanyAccess `gorm:"foreignKey:CompanyID" json:"employee_access,omitempty"`
}

// Bank is a global lookup table for Thai banks (not per-company)
type Bank struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_bank_code_deleted;not null;default:0" json:"-"`

	BankCode  string `gorm:"uniqueIndex:idx_bank_code_deleted;not null;size:10" json:"bank_code"` // e.g. BBL, KBANK
	BankName  string `gorm:"not null;size:200" json:"bank_name"`                                  // e.g. Bangkok Bank
	SwiftCode string `gorm:"size:20" json:"swift_code"`
}

// Holiday manages public and company-specific holidays per company
type Holiday struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_holiday_comp_date_deleted;not null;default:0" json:"-"`

	CompaniesID uint      `gorm:"uniqueIndex:idx_holiday_comp_date_deleted;not null;index" json:"companies_id"`
	Date        time.Time `gorm:"uniqueIndex:idx_holiday_comp_date_deleted;not null;type:date" json:"date"`
	Name        string    `gorm:"not null;size:200" json:"name"`
	IsPaid      bool      `gorm:"default:true" json:"is_paid"`

	Companies Companies `gorm:"foreignKey:CompaniesID" json:"companies"`
}

// DayOff manages company-specific planned days off (weekends/company off days)
type DayOff struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_dayoff_comp_weekday_deleted;not null;default:0" json:"-"`

	CompaniesID uint   `gorm:"uniqueIndex:idx_dayoff_comp_weekday_deleted;not null;index" json:"companies_id"`
	Weekday     int    `gorm:"uniqueIndex:idx_dayoff_comp_weekday_deleted;not null" json:"weekday"` // 0=Sunday ... 6=Saturday
	Name        string `gorm:"not null;size:200" json:"name"`
	IsPaid      bool   `gorm:"default:true" json:"is_paid"`

	Companies Companies `gorm:"foreignKey:CompaniesID" json:"companies"`
}

// WorkLocation lists branches or remote sites per company
type WorkLocation struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_wl_comp_name_deleted;not null;default:0" json:"-"`

	CompaniesID uint   `gorm:"uniqueIndex:idx_wl_comp_name_deleted;not null;index" json:"companies_id"`
	Name        string `gorm:"uniqueIndex:idx_wl_comp_name_deleted;not null;size:200" json:"name"`

	Companies Companies `gorm:"foreignKey:CompaniesID" json:"companies"`
}

// LeaveTypeSetting categorises leave requests per company (Sick, Annual, etc.)
// Named LeaveTypeSetting to avoid conflict with the LeaveType string-enum in attendance.go
type LeaveTypeSetting struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_lt_comp_name_deleted;not null;default:0" json:"-"`

	CompaniesID    uint    `gorm:"uniqueIndex:idx_lt_comp_name_deleted;not null;index" json:"companies_id"`
	Name           string  `gorm:"uniqueIndex:idx_lt_comp_name_deleted;not null;size:100" json:"name"`
	IsPaid         bool    `gorm:"default:true" json:"is_paid"`
	MaxDaysPerYear float64 `gorm:"type:decimal(5,2);default:0" json:"max_days_per_year"` // 0 = unlimited

	Companies Companies `gorm:"foreignKey:CompaniesID" json:"companies"`
}

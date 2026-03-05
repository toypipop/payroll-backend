package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type AttendanceStatus string

const (
	StatusOnTime  AttendanceStatus = "on_time"
	StatusLate    AttendanceStatus = "late"
	StatusAbsent  AttendanceStatus = "absent"
	StatusLeave   AttendanceStatus = "leave"
	StatusHoliday AttendanceStatus = "holiday"
)

type LeaveStatus string

const (
	LeaveStatusPending   LeaveStatus = "Pending"
	LeaveStatusApproved  LeaveStatus = "Approved"
	LeaveStatusRejected  LeaveStatus = "Rejected"
	LeaveStatusCancelled LeaveStatus = "Cancelled"
)

type LeaveType string

const (
	LeaveTypeSick     LeaveType = "Sick"
	LeaveTypePersonal LeaveType = "Personal"
	LeaveTypeAnnual   LeaveType = "Annual"
	LeaveTypeUnpaid   LeaveType = "Unpaid"
)

type TimeLog struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	DeletedAt   soft_delete.DeletedAt `gorm:"index" json:"-"`
	CompaniesID uint                  `gorm:"not null;index" json:"companies_id"`
	EmployeesID uint                  `gorm:"not null;index" json:"employees_id"`
	CheckIn     *time.Time            `json:"check_in"`
	CheckOut    *time.Time            `json:"check_out"`
	Status      AttendanceStatus      `gorm:"size:50;not null" json:"status" binding:"required,oneof=on_time late absent leave holiday"`
	LocationIn  string                `gorm:"size:255" json:"location_in"`
	Remark      string                `gorm:"size:255" json:"remark"`
	Employees   Employees             `gorm:"foreignKey:EmployeesID" json:"employees"`
}

type LeaveRequest struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	DeletedAt   soft_delete.DeletedAt `gorm:"index" json:"-"`
	CompaniesID uint                  `gorm:"not null;index" json:"companies_id"`
	EmployeesID uint                  `gorm:"not null;index" json:"employees_id"`
	LeaveType   LeaveType             `gorm:"not null;size:50" json:"leave_type" binding:"required,oneof=Sick Personal Annual Unpaid"`
	StartDate   time.Time             `gorm:"not null" json:"start_date"`
	EndDate     time.Time             `gorm:"not null" json:"end_date"`
	TotalDays   float64               `gorm:"type:decimal(5,2)" json:"total_days"`
	Reason      string                `gorm:"size:500" json:"reason"`
	Status      LeaveStatus           `gorm:"default:'Pending';size:50" json:"status" binding:"omitempty,oneof=Pending Approved Rejected Cancelled"`
	ApprovedBy  *uint                 `json:"approved_by"`
	Employees   Employees             `gorm:"foreignKey:EmployeesID" json:"employees"`
	Approver    *Employees            `gorm:"foreignKey:ApprovedBy" json:"approver"`
}

type LeaveBalance struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	DeletedAt   soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_year_type_deleted;not null;default:0" json:"-"`
	CompaniesID uint                  `gorm:"uniqueIndex:idx_emp_year_type_deleted;not null" json:"companies_id"`
	EmployeesID uint                  `gorm:"uniqueIndex:idx_emp_year_type_deleted;not null" json:"employees_id"`
	Year        int                   `gorm:"uniqueIndex:idx_emp_year_type_deleted;not null" json:"year"`
	LeaveType   LeaveType             `gorm:"uniqueIndex:idx_emp_year_type_deleted;not null;size:50" json:"leave_type"`
	MaxQuota    float64               `gorm:"type:decimal(5,2);default:0" json:"max_quota"`
	UsedQuota   float64               `gorm:"type:decimal(5,2);default:0" json:"used_quota"`
	Remaining   float64               `gorm:"type:decimal(5,2);default:0" json:"remaining"`
}

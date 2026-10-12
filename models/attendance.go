package models

import (
	"time"

	"gorm.io/gorm"
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
	ID             uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
	DeletedAt      soft_delete.DeletedAt `gorm:"index" json:"-"`
	CompaniesID    uint                  `gorm:"not null;index" json:"companies_id"`
	EmployeesID    uint                  `gorm:"not null;index" json:"employees_id"`
	CheckIn        *time.Time            `json:"check_in"`
	CheckOut       *time.Time            `json:"check_out"`
	Status         AttendanceStatus      `gorm:"size:50;not null;default:on_time" json:"status" binding:"omitempty,oneof=on_time late absent leave holiday"`
	WorkLocationID *uint                 `gorm:"not null" json:"work_location_id"` // FK: WorkLocation
	LocationIn     string                `gorm:"size:255" json:"location_in"`      // Legacy field
	Remark         string                `gorm:"size:255" json:"remark"`
	Employees      Employees             `gorm:"foreignKey:EmployeesID" json:"employees"`
	WorkLocation   *WorkLocation         `gorm:"foreignKey:WorkLocationID" json:"work_location"`
}

// AttendanceConfig for deduction rates
type AttendanceConfig struct {
	ID                        uint           `gorm:"primaryKey" json:"id"`
	CompaniesID               uint           `gorm:"not null;uniqueIndex" json:"companies_id"`
	LateRatePerMinute         float64        `gorm:"type:decimal(10,2);default:0" json:"late_rate_per_minute"`
	AbsentRatePerDay          float64        `gorm:"type:decimal(10,2);default:0" json:"absent_rate_per_day"`
	IsLateDeductionEnabled    bool           `gorm:"default:true" json:"is_late_deduction_enabled"`
	IsAbsentDeductionEnabled  bool           `gorm:"default:true" json:"is_absent_deduction_enabled"`
	WorkStartTime             *time.Time     `json:"work_start_time"`
	WorkEndTime               *time.Time     `json:"work_end_time"`
	OvertimeRatePerMinute     float64        `gorm:"type:decimal(10,2);default:0" json:"overtime_rate_per_minute"`
	ExtraRatePerDay           float64        `gorm:"type:decimal(10,2);default:0" json:"extra_rate_per_day"`
	IsOvertimeEarningsEnabled bool           `gorm:"default:false" json:"is_overtime_earnings_enabled"`
	IsExtraEarningsEnabled    bool           `gorm:"default:false" json:"is_extra_earnings_enabled"`
	CreatedAt                 time.Time      `json:"created_at"`
	UpdatedAt                 time.Time      `json:"updated_at"`
	DeletedAt                 gorm.DeletedAt `gorm:"index" json:"-"`
	Companies                 Companies      `gorm:"foreignKey:CompaniesID" json:"companies"`
}

type LeaveRequest struct {
	ID              uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	DeletedAt       soft_delete.DeletedAt `gorm:"index" json:"-"`
	CompaniesID     uint                  `gorm:"not null;index" json:"companies_id"`
	EmployeesID     uint                  `gorm:"not null;index" json:"employees_id"`
	LeaveTypeID     *uint                 `json:"leave_type_id"` // FK: LeaveTypeSetting
	LeaveType       LeaveType             `gorm:"not null;size:100" json:"leave_type"`
	StartDate       time.Time             `gorm:"not null" json:"start_date"`
	EndDate         time.Time             `gorm:"not null" json:"end_date"`
	TotalDays       float64               `gorm:"type:decimal(5,2)" json:"total_days"`
	Reason          string                `gorm:"size:500" json:"reason"`
	Status          LeaveStatus           `gorm:"default:'Pending';size:50" json:"status" binding:"omitempty,oneof=Pending Approved Rejected Cancelled"`
	ApprovedBy      *uint                 `json:"approved_by"`
	Employees       Employees             `gorm:"foreignKey:EmployeesID" json:"employees"`
	Approver        *Employees            `gorm:"foreignKey:ApprovedBy" json:"approver"`
	LeaveTypeMaster *LeaveTypeSetting     `gorm:"foreignKey:LeaveTypeID" json:"leave_type_master"`
}

type LeaveBalance struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	DeletedAt   soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_year_type_deleted;not null;default:0" json:"-"`
	CompaniesID uint                  `gorm:"uniqueIndex:idx_emp_year_type_deleted;not null" json:"companies_id"`
	EmployeesID uint                  `gorm:"uniqueIndex:idx_emp_year_type_deleted;not null" json:"employees_id"`
	Year        int                   `gorm:"uniqueIndex:idx_emp_year_type_deleted;not null" json:"year"`
	LeaveType   LeaveType             `gorm:"uniqueIndex:idx_emp_year_type_deleted;not null;size:100" json:"leave_type"`
	MaxQuota    float64               `gorm:"type:decimal(5,2);default:0" json:"max_quota"`
	UsedQuota   float64               `gorm:"type:decimal(5,2);default:0" json:"used_quota"`
	Remaining   float64               `gorm:"type:decimal(5,2);default:0" json:"remaining"`
}

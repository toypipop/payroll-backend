package models

import "time"

type SalaryStructure struct {
	EmployeeID uint    `gorm:"primaryKey" json:"employee_id"`
	BaseSalary float64 `gorm:"type:decimal(15,2)" json:"base_salary"`
}

type PayrollCycle struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CycleName string    `json:"cycle_name"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	IsClosed  bool      `gorm:"default:false" json:"is_closed"`
}

type PayrollTransaction struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CycleID      uint      `json:"cycle_id"`
	EmployeeID   uint      `json:"employee_id"`
	SalaryAtTime float64   `gorm:"type:decimal(15,2)" json:"salary_at_time"`
	NetPay       float64   `gorm:"type:decimal(15,2)" json:"net_pay"`
	CreatedAt    time.Time `json:"created_at"`
}

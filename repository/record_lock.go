package repository

import (
	"errors"
	"fmt"
	"payroll-backend/config"
)

var (
	// ErrRecordInUse is returned when a transactional reference record (earning type, deduction
	// type, department, position) appears in at least one closed payroll cycle and therefore
	// cannot be deleted.
	ErrRecordInUse = errors.New("record is in use by closed payroll transactions")

	// ErrGlobalSettingLocked is returned when a company-level global setting (PayrollConfig,
	// AttendanceConfig) cannot be edited because the company already has at least one closed
	// payroll cycle. Once a cycle is closed those settings are permanently read-only.
	ErrGlobalSettingLocked = errors.New("settings are read-only: company has at least one closed payroll cycle")
)

// IsRecordLocked reports whether a deletable master record is directly referenced in at least
// one closed payroll cycle transaction, making deletion of that record strictly forbidden.
//
// Supported tables: earning_type, deduction_type, department, position.
//
// Note: company-level global settings (payroll_config, attendance_config) are NOT managed here;
// use HasClosedCycleForCompany for those.
func IsRecordLocked(table string, id uint) (bool, error) {
	var count int64

	switch table {
	case "earning_type":
		err := config.DB.Table("payroll_transaction_detail ptd").
			Joins("JOIN payroll_transaction pt ON pt.id = ptd.transaction_id").
			Joins("JOIN payroll_cycle pc ON pc.id = pt.cycle_id").
			Where("ptd.earning_type_id = ? AND pc.is_closed = ?", id, true).
			Count(&count).Error
		return count > 0, err

	case "deduction_type":
		err := config.DB.Table("payroll_transaction_detail ptd").
			Joins("JOIN payroll_transaction pt ON pt.id = ptd.transaction_id").
			Joins("JOIN payroll_cycle pc ON pc.id = pt.cycle_id").
			Where("ptd.deduction_type_id = ? AND pc.is_closed = ?", id, true).
			Count(&count).Error
		return count > 0, err

	case "department":
		err := config.DB.Table("payroll_transaction pt").
			Joins("JOIN payroll_cycle pc ON pc.id = pt.cycle_id").
			Joins("JOIN employees e ON e.id = pt.employees_id").
			Where("e.dept_id = ? AND pc.is_closed = ?", id, true).
			Count(&count).Error
		return count > 0, err

	case "position":
		err := config.DB.Table("payroll_transaction pt").
			Joins("JOIN payroll_cycle pc ON pc.id = pt.cycle_id").
			Joins("JOIN employees e ON e.id = pt.employees_id").
			Where("e.pos_id = ? AND pc.is_closed = ?", id, true).
			Count(&count).Error
		return count > 0, err

	default:
		return false, fmt.Errorf("unsupported lock table: %s", table)
	}
}

// HasClosedCycleForCompany is the canonical read-only guard for all company-level global
// settings (PayrollConfig, AttendanceConfig). It returns true if the company has at least one
// closed payroll cycle, meaning every global setting for that company must be treated as
// read-only and must not be updated.
func HasClosedCycleForCompany(companiesID uint) (bool, error) {
	var count int64
	err := config.DB.Table("payroll_cycle").
		Where("companies_id = ? AND is_closed = ?", companiesID, true).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

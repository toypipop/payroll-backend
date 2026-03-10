package repository

import (
	"errors"
	"fmt"
	"payroll-backend/config"
)

var ErrRecordInUse = errors.New("record is in use by closed payroll transactions")

// IsRecordLocked checks if a master/config record has been referenced by any closed payroll transaction.
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

	case "attendance_config":
		var companiesID uint
		if err := config.DB.Table("attendance_config").Select("companies_id").Where("id = ?", id).Scan(&companiesID).Error; err != nil {
			return false, err
		}
		if companiesID == 0 {
			return false, nil
		}
		err := config.DB.Table("payroll_transaction pt").
			Joins("JOIN payroll_cycle pc ON pc.id = pt.cycle_id").
			Where("pc.companies_id = ? AND pc.is_closed = ?", companiesID, true).
			Count(&count).Error
		return count > 0, err

	case "payroll_config":
		var companiesID uint
		if err := config.DB.Table("payroll_config").Select("companies_id").Where("id = ?", id).Scan(&companiesID).Error; err != nil {
			return false, err
		}
		if companiesID == 0 {
			return false, nil
		}
		err := config.DB.Table("payroll_cycle").
			Where("companies_id = ? AND is_closed = ?", companiesID, true).
			Count(&count).Error
		return count > 0, err

	default:
		return false, fmt.Errorf("unsupported lock table: %s", table)
	}
}

// HasClosedCycleForCompany is used to lock global calculation settings once payroll periods are closed.
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

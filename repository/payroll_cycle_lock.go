package repository

import (
	"errors"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

var (
	ErrPayrollCycleNotFound = errors.New("payroll cycle not found or access denied")
	ErrPayrollCycleClosed   = errors.New("payroll cycle is closed")
)

// EnsurePayrollCycleEditable validates that cycle exists in company and is not closed.
func EnsurePayrollCycleEditable(cycleID uint, companiesID uint) error {
	var cycle models.PayrollCycle
	if err := config.DB.Where("id = ? AND companies_id = ?", cycleID, companiesID).First(&cycle).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPayrollCycleNotFound
		}
		return err
	}
	if cycle.IsClosed {
		return ErrPayrollCycleClosed
	}
	return nil
}

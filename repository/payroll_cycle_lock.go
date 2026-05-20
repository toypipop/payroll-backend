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

// EnsurePayrollCycleOpen validates the cycle is accessible and open.
// Uses ApplyScopes so mapped role access controls which company cycles are visible.
// Returns the cycle so callers can use cycle.CompaniesID.
func EnsurePayrollCycleOpen(cycleID uint, user models.Employees, scope models.DataScope) (*models.PayrollCycle, error) {
	var cycle models.PayrollCycle
	if err := config.DB.
		Scopes(ApplyScopes(user, scope, "payroll_cycle.companies_id", "unused")).
		First(&cycle, cycleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPayrollCycleNotFound
		}
		return nil, err
	}
	if cycle.IsClosed {
		return nil, ErrPayrollCycleClosed
	}
	return &cycle, nil
}

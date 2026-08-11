package repository

import (
	"errors"
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	ErrPayrollCycleNotFound = errors.New("payroll cycle not found or access denied")
	ErrPayrollCycleClosed   = errors.New("payroll cycle is closed")
)

// EnsurePayrollCycleOpen validates the cycle is accessible and open.
// A payroll cycle belongs to a company, not a person, so this is company-scoped
// and takes no DataScope: the cycle must sit in a company the caller's role
// actually grants this action on, not merely one they can reach.
// Returns the cycle so callers can use cycle.CompaniesID.
func EnsurePayrollCycleOpen(c *gin.Context, cycleID uint, user models.Employees) (*models.PayrollCycle, error) {
	var cycle models.PayrollCycle
	if err := config.DB.
		Scopes(ApplyCompanyScopes(c, user, "payroll_cycle.companies_id")).
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

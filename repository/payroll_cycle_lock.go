package repository

import (
	"errors"
	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Carry apperr kinds so a migrated handler gets the right status from httpx.
// Handlers not yet migrated compare these by identity and write their own
// messages, so giving them a kind changes nothing for those call sites.
var (
	ErrPayrollCycleNotFound = apperr.NotFound("Payroll cycle not found or access denied")
	ErrPayrollCycleClosed   = apperr.Conflict("Payroll cycle is closed")
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

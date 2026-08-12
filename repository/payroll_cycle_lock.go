package repository

import "payroll-backend/apperr"

// Carry apperr kinds so a migrated handler gets the right status from httpx.
// Handlers not yet migrated compare these by identity and write their own
// messages, so giving them a kind changes nothing for those call sites.
var (
	ErrPayrollCycleNotFound = apperr.NotFound("Payroll cycle not found or access denied")
	ErrPayrollCycleClosed   = apperr.Conflict("Payroll cycle is closed")
)

// The gin-based EnsurePayrollCycleOpen that used to live here is gone. Its callers
// now go through PayrollRunRepository.FindOpenCycleScoped, which takes an AccessScope
// instead of a *gin.Context. The sentinels stay because both the payroll-run and
// payroll-config repositories answer with them.

package repository

import "payroll-backend/apperr"

// Carry apperr kinds so a migrated handler gets the right status from httpx.
var (
	ErrPayrollCycleNotFound = apperr.NotFound("Payroll cycle not found or access denied")
	ErrPayrollCycleClosed   = apperr.Conflict("Payroll cycle is closed")
)

// The gin-based EnsurePayrollCycleOpen that used to live here is gone.

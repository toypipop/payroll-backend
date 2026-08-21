package payroll

import "payroll-backend/service"

// Handler carries the services this package's routes need, built once in main so no handler reaches for a database handle of its own.
type Handler struct {
	PayrollRun     *service.PayrollRunService
	PayrollConfig  *service.PayrollConfigService
	CycleEarning   *service.CycleEarningService
	CycleDeduction *service.CycleDeductionService
}

package payrollconfig

import "payroll-backend/service"

// Handler carries the services this package's routes need, built once in main so no handler reaches for a database handle of its own.
type Handler struct {
	PayrollConfig  *service.PayrollConfigService
	EarningType    *service.EarningTypeService
	DeductionType  *service.DeductionTypeService
	EmploymentType *service.EmploymentTypeService
}

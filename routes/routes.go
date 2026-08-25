package routes

import (
	"os"
	atthandler "payroll-backend/handlers/attendance"
	authhandler "payroll-backend/handlers/auth"
	emphandler "payroll-backend/handlers/employees"
	payhandler "payroll-backend/handlers/payroll"
	cfghandler "payroll-backend/handlers/payrollconfig"
	rephandler "payroll-backend/handlers/reports"
	sethandler "payroll-backend/handlers/settings"
	"payroll-backend/middleware"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func allowedOrigins() []string {
	origins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if strings.TrimSpace(origins) == "" {
		origins = os.Getenv("FRONTEND_ORIGIN")
	}
	if strings.TrimSpace(origins) == "" {
		return []string{"http://localhost:5174"}
	}

	result := []string{}
	for _, origin := range strings.Split(origins, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			result = append(result, origin)
		}
	}
	if len(result) == 0 {
		return []string{"http://localhost:5174"}
	}
	return result
}

// Handlers bundles one handler value per package, built in main so routing stays a pure wiring step.
type Handlers struct {
	Attendance    *atthandler.Handler
	Auth          *authhandler.Handler
	Employees     *emphandler.Handler
	Payroll       *payhandler.Handler
	PayrollConfig *cfghandler.Handler
	Reports       *rephandler.Handler
	Settings      *sethandler.Handler
}

func SetupRouter(h Handlers, g *middleware.Guard) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Public Routes
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.POST("/login", h.Auth.Login)
		authGroup.POST("/logout", h.Auth.Logout)
		authGroup.GET("/me", g.Auth(), h.Auth.Me)
	}
	r.GET("/api/v1/modules", h.Auth.GetAppModules)
	r.GET("/api/v1/modules/mine", g.Auth(), h.Auth.GetMyAppModules)

	// Protected Routes
	api := r.Group("/api/v1")
	api.Use(g.Auth())
	api.Use(middleware.RequireXMLHttpRequest())
	{
		// --- Employees ---
		api.GET("/employees", g.CheckPermission("EmployeesListPage", "VIEW"), h.Employees.GetEmployees)
		api.POST("/employees", g.CheckPermission("EmployeesListPage", "CREATE"), h.Employees.CreateEmployees)
		api.PUT("/employees/:id", g.CheckPermission("EmployeesListPage", "EDIT"), h.Employees.UpdateEmployees)
		api.DELETE("/employees/:id", g.CheckPermission("EmployeesListPage", "DELETE"), h.Employees.DeleteEmployees)

		api.GET("/salaries", g.CheckPermission("SalariesStructurePage", "VIEW"), h.Employees.GetSalaries)
		api.POST("/salaries", g.CheckPermission("SalariesStructurePage", "CREATE"), h.Employees.UpdateSalariesStructure)
		api.GET("/salaries/history/:employees_id", g.CheckPermission("SalariesStructurePage", "VIEW"), h.Employees.GetSalaryHistory)
		api.PATCH("/salaries/:id", g.CheckPermission("SalariesStructurePage", "EDIT"), h.Employees.UpdateSalaryRecord)
		api.DELETE("/salaries/:id", g.CheckPermission("SalariesStructurePage", "DELETE"), h.Employees.DeleteSalaryRecord)

		api.GET("/employee-earnings", g.CheckPermission("EmployeeEarningsPage", "VIEW"), h.Employees.GetEmployeeEarnings)
		api.POST("/employee-earnings", g.CheckPermission("EmployeeEarningsPage", "CREATE"), h.Employees.CreateEmployeeEarning)
		api.PUT("/employee-earnings/:id", g.CheckPermission("EmployeeEarningsPage", "EDIT"), h.Employees.UpdateEmployeeEarning)
		api.DELETE("/employee-earnings/:id", g.CheckPermission("EmployeeEarningsPage", "DELETE"), h.Employees.DeleteEmployeeEarning)

		api.GET("/employee-deductions", g.CheckPermission("EmployeeDeductionsPage", "VIEW"), h.Employees.GetEmployeeDeductions)
		api.POST("/employee-deductions", g.CheckPermission("EmployeeDeductionsPage", "CREATE"), h.Employees.CreateEmployeeDeduction)
		api.PUT("/employee-deductions/:id", g.CheckPermission("EmployeeDeductionsPage", "EDIT"), h.Employees.UpdateEmployeeDeduction)
		api.DELETE("/employee-deductions/:id", g.CheckPermission("EmployeeDeductionsPage", "DELETE"), h.Employees.DeleteEmployeeDeduction)

		// --- Attendance ---
		api.GET("/timelogs", g.CheckPermission("TimelogsListPage", "VIEW"), h.Attendance.GetTimeLogs)
		api.POST("/timelogs", g.CheckPermission("TimelogsListPage", "CREATE"), h.Attendance.CreateTimeLog)
		api.POST("/timelogs/bulk", g.CheckPermission("TimelogsListPage", "CREATE"), h.Attendance.BulkCreateTimeLog)
		api.PUT("/timelogs/:id", g.CheckPermission("TimelogsListPage", "EDIT"), h.Attendance.UpdateTimeLog)
		api.DELETE("/timelogs/:id", g.CheckPermission("TimelogsListPage", "DELETE"), h.Attendance.DeleteTimeLog)
		api.GET("/timelogs/schedule-config", g.CheckPermission("TimelogsListPage", "VIEW"), h.Attendance.GetTimelogScheduleConfig)

		api.GET("/leaves/requests", g.CheckPermission("LeaveRequestsPage", "VIEW"), h.Attendance.GetLeaveRequests)
		api.POST("/leaves/requests", g.CheckPermission("LeaveRequestsPage", "CREATE"), h.Attendance.CreateLeaveRequest)
		api.PUT("/leaves/requests/:id", g.CheckPermission("LeaveRequestsPage", "EDIT"), h.Attendance.UpdateLeaveRequest)
		api.DELETE("/leaves/requests/:id", g.CheckPermission("LeaveRequestsPage", "DELETE"), h.Attendance.DeleteLeaveRequest)

		api.GET("/leaves/approved", g.CheckPermission("LeaveApprovedPage", "VIEW"), h.Attendance.GetPendingLeaveRequests)
		api.PUT("/leaves/approve/:id", g.CheckPermission("LeaveApprovedPage", "EDIT"), h.Attendance.ApproveLeaveRequest)

		api.GET("/attendance/config", g.CheckPermission("AttendanceSettingsPage", "VIEW"), h.Attendance.GetAttendanceConfigs)
		api.POST("/attendance/config", g.CheckPermission("AttendanceSettingsPage", "EDIT"), h.Attendance.UpdateAttendanceConfig)

		api.GET("/deduction/summary", g.CheckPermission("DeductionSummaryPage", "VIEW"), h.Attendance.GetAttendanceSummary)
		api.GET("/earning/summary", g.CheckPermission("EarningSummaryPage", "VIEW"), h.Attendance.GetEarningSummary)

		// --- Payroll Processing ---
		api.GET("/payroll/cycles", g.CheckPermission("PayrollCyclesPage", "VIEW"), h.Payroll.GetPayrollCycles)
		api.POST("/payroll/cycles", g.CheckPermission("PayrollCyclesPage", "CREATE"), h.Payroll.CreatePayrollCycle)
		api.PATCH("/payroll/cycles/:id/close", g.CheckPermission("PayrollCyclesPage", "EDIT"), h.Payroll.ClosePayrollCycle)
		api.GET("/payroll/pre-check/:cycle_id", g.CheckPermission("RunPayrollPage", "EDIT"), h.Payroll.PreCheckPayroll)
		api.GET("/payroll/diff/:cycle_id", g.CheckPermission("RunPayrollPage", "VIEW"), h.Payroll.GetPayrollDiff)
		api.POST("/payroll/calculate/:cycle_id", g.CheckPermission("RunPayrollPage", "EDIT"), h.Payroll.CalculatePayroll)
		// One cycle's transactions are read by four pages. Each gets its own route so the gate names the page that asked,
		// rather than one route accepting whichever of four permissions the caller happens to hold.
		api.GET("/payroll/transactions/:cycle_id", g.CheckPermission("TransactionsPage", "VIEW"), h.Payroll.GetPayrollTransactions)
		api.GET("/payroll/run/transactions/:cycle_id", g.CheckPermission("RunPayrollPage", "VIEW"), h.Payroll.GetPayrollTransactions)
		api.GET("/reports/payslips/transactions/:cycle_id", g.CheckPermission("PayslipsReportPage", "VIEW"), h.Payroll.GetPayrollTransactions)
		api.GET("/reports/tax-forms/transactions/:cycle_id", g.CheckPermission("TaxFormsReportPage", "VIEW"), h.Payroll.GetPayrollTransactions)
		api.GET("/payroll/slip/:transaction_id", g.CheckPermission("TransactionsPage", "VIEW"), h.Payroll.GetPayrollSlip)
		api.GET("/payroll/my-slips", h.Payroll.GetMySlips)

		api.GET("/cycle-earnings", g.CheckPermission("CycleEarningsPage", "VIEW"), h.Payroll.GetCycleEarnings)
		api.POST("/cycle-earnings", g.CheckPermission("CycleEarningsPage", "CREATE"), h.Payroll.CreateCycleEarning)
		api.PUT("/cycle-earnings/:id", g.CheckPermission("CycleEarningsPage", "EDIT"), h.Payroll.UpdateCycleEarning)
		api.DELETE("/cycle-earnings/:id", g.CheckPermission("CycleEarningsPage", "DELETE"), h.Payroll.DeleteCycleEarning)

		api.GET("/cycle-deductions", g.CheckPermission("CycleDeductionsPage", "VIEW"), h.Payroll.GetCycleDeductions)
		api.POST("/cycle-deductions", g.CheckPermission("CycleDeductionsPage", "CREATE"), h.Payroll.CreateCycleDeduction)
		api.PUT("/cycle-deductions/:id", g.CheckPermission("CycleDeductionsPage", "EDIT"), h.Payroll.UpdateCycleDeduction)
		api.DELETE("/cycle-deductions/:id", g.CheckPermission("CycleDeductionsPage", "DELETE"), h.Payroll.DeleteCycleDeduction)

		// --- Payroll Config ---
		api.GET("/global", g.CheckPermission("PayrollGlobalSettingsPage", "VIEW"), h.Payroll.GetPayrollConfigs)
		api.POST("/global", g.CheckPermission("PayrollGlobalSettingsPage", "EDIT"), h.Payroll.UpdatePayrollConfig)
		api.GET("/payroll/config/:cycle_id", g.CheckPermission("RunPayrollPage", "VIEW"), h.PayrollConfig.GetConfigByPayrollCycle)

		api.GET("/earning-types", g.CheckPermission("EarningTypesPage", "VIEW"), h.PayrollConfig.GetEarningTypes)
		// No CheckPermission gate: handler limits results to caller's accessible companies, and any role assigning employee/cycle earnings needs this dropdown.
		api.GET("/earning-types/options", h.PayrollConfig.GetEarningTypeOptions)
		api.POST("/earning-types", g.CheckPermission("EarningTypesPage", "CREATE"), h.PayrollConfig.CreateEarningType)
		api.PUT("/earning-types/:id", g.CheckPermission("EarningTypesPage", "EDIT"), h.PayrollConfig.UpdateEarningType)
		api.DELETE("/earning-types/:id", g.CheckPermission("EarningTypesPage", "EDIT"), g.PreventDeleteIfReferenced("earning_type"), h.PayrollConfig.DeleteEarningType)

		api.GET("/deduction-types", g.CheckPermission("DeductionTypesPage", "VIEW"), h.PayrollConfig.GetDeductionTypes)
		// No CheckPermission gate: handler limits results to caller's accessible companies, and any role assigning employee/cycle deductions needs this dropdown.
		api.GET("/deduction-types/options", h.PayrollConfig.GetDeductionTypeOptions)
		api.POST("/deduction-types", g.CheckPermission("DeductionTypesPage", "EDIT"), h.PayrollConfig.CreateDeductionType)
		api.PUT("/deduction-types/:id", g.CheckPermission("DeductionTypesPage", "EDIT"), h.PayrollConfig.UpdateDeductionType)
		api.DELETE("/deduction-types/:id", g.CheckPermission("DeductionTypesPage", "EDIT"), g.PreventDeleteIfReferenced("deduction_type"), h.PayrollConfig.DeleteDeductionType)

		api.GET("/master/employment-types", g.CheckPermission("EmploymentTypePage", "VIEW"), h.PayrollConfig.GetEmploymentTypes)
		// No CheckPermission gate: handler limits results to caller's accessible companies, and any role filling out an employee-linked form needs this dropdown.
		api.GET("/master/employment-types/options", h.PayrollConfig.GetEmploymentTypeOptions)
		api.POST("/master/employment-types", g.CheckPermission("EmploymentTypePage", "CREATE"), h.PayrollConfig.CreateEmploymentType)
		api.PUT("/master/employment-types/:id", g.CheckPermission("EmploymentTypePage", "EDIT"), h.PayrollConfig.UpdateEmploymentType)
		api.DELETE("/master/employment-types/:id", g.CheckPermission("EmploymentTypePage", "DELETE"), g.PreventDeleteIfReferenced("employment_type"), h.PayrollConfig.DeleteEmploymentType)

		// --- Settings ---
		api.GET("/departments", g.CheckPermission("DepartmentSettings", "VIEW"), h.Settings.GetDepartments)
		// No CheckPermission gate: handler limits results to caller's accessible companies, and any role filling out an employee-linked form needs this dropdown.
		api.GET("/departments/options", h.Settings.GetDepartmentOptions)
		api.POST("/departments", g.CheckPermission("DepartmentSettings", "CREATE"), h.Settings.CreateDepartment)
		api.PUT("/departments/:id", g.CheckPermission("DepartmentSettings", "EDIT"), h.Settings.UpdateDepartment)
		api.DELETE("/departments/:id", g.CheckPermission("DepartmentSettings", "DELETE"), g.PreventDeleteIfReferenced("department"), h.Settings.DeleteDepartment)
		api.GET("/department-categories", g.CheckPermission("DepartmentSettings", "VIEW"), h.Settings.GetDepartmentCategories)
		api.POST("/department-categories", g.CheckPermission("DepartmentSettings", "CREATE"), h.Settings.CreateDepartmentCategory)

		api.GET("/positions", g.CheckPermission("PositionSettings", "VIEW"), h.Settings.GetPositions)
		// No CheckPermission gate: handler limits results to caller's accessible companies, and any role filling out an employee-linked form needs this dropdown.
		api.GET("/positions/options", h.Settings.GetPositionOptions)
		api.POST("/positions", g.CheckPermission("PositionSettings", "CREATE"), h.Settings.CreatePosition)
		api.PUT("/positions/:id", g.CheckPermission("PositionSettings", "EDIT"), h.Settings.UpdatePosition)
		api.DELETE("/positions/:id", g.CheckPermission("PositionSettings", "DELETE"), g.PreventDeleteIfReferenced("position"), h.Settings.DeletePosition)

		api.GET("/roles", g.CheckPermission("RolesPage", "VIEW"), h.Settings.GetRoles)
		// No CheckPermission gate: handler limits results to caller's accessible
		api.GET("/roles/options", h.Settings.GetRoleOptions)
		api.POST("/roles", g.CheckPermission("RolesPage", "EDIT"), h.Settings.CreateRole)
		api.PUT("/roles/:id", g.CheckPermission("RolesPage", "EDIT"), h.Settings.UpdateRole)
		api.DELETE("/roles/:id", g.CheckPermission("RolesPage", "EDIT"), g.PreventDeleteIfReferenced("role"), h.Settings.DeleteRole)

		api.GET("/permissions/:id", g.CheckPermission("PermissionSettings", "VIEW"), h.Settings.GetRolesPermissions)
		api.POST("/permissions/:id", g.CheckPermission("PermissionSettings", "EDIT"), h.Settings.UpdateRolesPermissions)

		api.GET("/employee-company-access", g.CheckPermission("EmployeeCompanyAccessPage", "VIEW"), h.Settings.GetEmployeeCompanyAccess)
		api.POST("/employee-company-access", g.CheckPermission("EmployeeCompanyAccessPage", "CREATE"), h.Settings.CreateEmployeeCompanyAccess)
		api.PUT("/employee-company-access/:id", g.CheckPermission("EmployeeCompanyAccessPage", "EDIT"), h.Settings.UpdateEmployeeCompanyAccess)
		api.DELETE("/employee-company-access/:id", g.CheckPermission("EmployeeCompanyAccessPage", "DELETE"), h.Settings.DeleteEmployeeCompanyAccess)

		api.GET("/companies/all", g.CheckPermission("CompaniesPage", "VIEW"), h.Settings.GetAllCompanies)
		// No CheckPermission gate: the handler already limits the result to the caller's accessible companies, and every role needs this dropdown.
		api.GET("/companies/options", h.Settings.GetCompanies)
		api.POST("/companies", g.CheckPermission("CompaniesPage", "CREATE"), h.Settings.CreateCompanies)
		api.PUT("/companies/:id", g.CheckPermission("CompaniesPage", "EDIT"), h.Settings.UpdateCompanies)
		api.DELETE("/companies/:id", g.CheckPermission("CompaniesPage", "DELETE"), g.PreventDeleteIfReferenced("companies"), h.Settings.DeleteCompanies)

		api.GET("/master/banks", g.CheckPermission("MasterBankPage", "VIEW"), h.Settings.GetBanks)
		// No CheckPermission gate: the handler returns global reference data, not company rows.
		api.GET("/master/banks/options", h.Settings.GetBankOptions)
		api.POST("/master/banks", g.CheckPermission("MasterBankPage", "CREATE"), h.Settings.CreateBank)
		api.PUT("/master/banks/:id", g.CheckPermission("MasterBankPage", "EDIT"), h.Settings.UpdateBank)
		api.DELETE("/master/banks/:id", g.CheckPermission("MasterBankPage", "DELETE"), g.PreventDeleteIfReferenced("bank"), h.Settings.DeleteBank)

		api.GET("/master/holidays", g.CheckPermission("HolidayCalendarPage", "VIEW"), h.Settings.GetHolidays)
		api.POST("/master/holidays", g.CheckPermission("HolidayCalendarPage", "CREATE"), h.Settings.CreateHoliday)
		api.PUT("/master/holidays/:id", g.CheckPermission("HolidayCalendarPage", "EDIT"), h.Settings.UpdateHoliday)
		api.DELETE("/master/holidays/:id", g.CheckPermission("HolidayCalendarPage", "DELETE"), g.PreventDeleteIfReferenced("holiday"), h.Settings.DeleteHoliday)

		api.GET("/master/day-offs", g.CheckPermission("DayOffPage", "VIEW"), h.Settings.GetDayOffs)
		api.GET("/master/day-offs/options", h.Settings.GetDayOffOptions)
		api.GET("/master/day-offs/companies", g.CheckPermission("DayOffPage", "VIEW"), h.Settings.GetDayOffCompanies)
		api.POST("/master/day-offs", g.CheckPermission("DayOffPage", "CREATE"), h.Settings.CreateDayOff)
		api.PUT("/master/day-offs/:id", g.CheckPermission("DayOffPage", "EDIT"), h.Settings.UpdateDayOff)
		api.DELETE("/master/day-offs/:id", g.CheckPermission("DayOffPage", "DELETE"), h.Settings.DeleteDayOff)

		api.GET("/master/work-locations", g.CheckPermission("WorkLocationPage", "VIEW"), h.Settings.GetWorkLocations)
		// No CheckPermission gate: handler limits results to caller's accessible companies, and any role filling out a time log needs this dropdown.
		api.GET("/master/work-locations/options", h.Settings.GetWorkLocationOptions)
		api.POST("/master/work-locations", g.CheckPermission("WorkLocationPage", "CREATE"), h.Settings.CreateWorkLocation)
		api.PUT("/master/work-locations/:id", g.CheckPermission("WorkLocationPage", "EDIT"), h.Settings.UpdateWorkLocation)
		api.DELETE("/master/work-locations/:id", g.CheckPermission("WorkLocationPage", "DELETE"), g.PreventDeleteIfReferenced("work_location"), h.Settings.DeleteWorkLocation)

		api.GET("/master/leave-types", g.CheckPermission("MasterLeaveTypePage", "VIEW"), h.Settings.GetLeaveTypeSettings)
		// No CheckPermission gate: handler limits results to caller's accessible companies, and any role submitting a leave request needs this dropdown.
		api.GET("/master/leave-types/options", h.Settings.GetLeaveTypeOptions)
		api.POST("/master/leave-types", g.CheckPermission("MasterLeaveTypePage", "CREATE"), h.Settings.CreateLeaveTypeSetting)
		api.PUT("/master/leave-types/:id", g.CheckPermission("MasterLeaveTypePage", "EDIT"), h.Settings.UpdateLeaveTypeSetting)
		api.DELETE("/master/leave-types/:id", g.CheckPermission("MasterLeaveTypePage", "DELETE"), g.PreventDeleteIfReferenced("leave_type"), h.Settings.DeleteLeaveTypeSetting)

		// --- Reports ---
		api.GET("/payslips/:transaction_id", g.CheckPermission("PayslipsReportPage", "VIEW"), h.Reports.GetPayslipReport)
		api.GET("/reports/bank-transfer/:cycle_id", g.CheckPermission("BankTransferReportPage", "VIEW"), h.Reports.GetBankTransferReport)
		api.GET("/reports/tax-forms/pnd1/:cycle_id", g.CheckPermission("TaxFormsReportPage", "VIEW"), h.Reports.GetTaxFormsPND1)
		api.GET("/reports/tax-forms/50tawi/:transaction_id", g.CheckPermission("TaxFormsReportPage", "VIEW"), h.Reports.GetTax50Tawi)
		api.GET("/reports/sso/:cycle_id", g.CheckPermission("SSOReportPage", "VIEW"), h.Reports.GetSSOReport)
		api.GET("/reports/payroll-analysis/:cycle_id", g.CheckPermission("PayrollAnalysisPage", "VIEW"), h.Reports.GetPayrollAnalysis)
	}
	return r
}

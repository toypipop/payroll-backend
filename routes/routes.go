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

func SetupRouter() *gin.Engine {
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
		authGroup.POST("/login", authhandler.Login)
		authGroup.POST("/logout", authhandler.Logout)
		authGroup.GET("/me", middleware.AuthMiddleware(), authhandler.Me)
	}
	r.GET("/api/v1/modules", authhandler.GetAppModules)

	// Protected Routes
	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware())
	api.Use(middleware.RequireXMLHttpRequest())
	{
		// --- Employees ---
		api.GET("/employees", middleware.CheckPermission("EmployeesListPage", "VIEW"), emphandler.GetEmployees)
		api.POST("/employees", middleware.CheckPermission("EmployeesListPage", "CREATE"), emphandler.CreateEmployees)
		api.PUT("/employees/:id", middleware.CheckPermission("EmployeesListPage", "EDIT"), emphandler.UpdateEmployees)
		api.DELETE("/employees/:id", middleware.CheckPermission("EmployeesListPage", "DELETE"), emphandler.DeleteEmployees)

		api.GET("/salaries", middleware.CheckPermission("SalariesStructurePage", "VIEW"), emphandler.GetSalaries)
		api.POST("/salaries", middleware.CheckPermission("SalariesStructurePage", "CREATE"), emphandler.UpdateSalariesStructure)

		api.GET("/employee-earnings", middleware.CheckPermission("EmployeeEarningsPage", "VIEW"), emphandler.GetEmployeeEarnings)
		api.POST("/employee-earnings", middleware.CheckPermission("EmployeeEarningsPage", "CREATE"), emphandler.CreateEmployeeEarning)
		api.PUT("/employee-earnings/:id", middleware.CheckPermission("EmployeeEarningsPage", "EDIT"), emphandler.UpdateEmployeeEarning)
		api.DELETE("/employee-earnings/:id", middleware.CheckPermission("EmployeeEarningsPage", "DELETE"), emphandler.DeleteEmployeeEarning)

		api.GET("/employee-deductions", middleware.CheckPermission("EmployeeDeductionsPage", "VIEW"), emphandler.GetEmployeeDeductions)
		api.POST("/employee-deductions", middleware.CheckPermission("EmployeeDeductionsPage", "CREATE"), emphandler.CreateEmployeeDeduction)
		api.PUT("/employee-deductions/:id", middleware.CheckPermission("EmployeeDeductionsPage", "EDIT"), emphandler.UpdateEmployeeDeduction)
		api.DELETE("/employee-deductions/:id", middleware.CheckPermission("EmployeeDeductionsPage", "DELETE"), emphandler.DeleteEmployeeDeduction)

		// --- Attendance ---
		api.GET("/timelogs", middleware.CheckPermission("TimelogsListPage", "VIEW"), atthandler.GetTimeLogs)
		api.POST("/timelogs", middleware.CheckPermission("TimelogsListPage", "CREATE"), atthandler.CreateTimeLog)
		api.PUT("/timelogs/:id", middleware.CheckPermission("TimelogsListPage", "EDIT"), atthandler.UpdateTimeLog)
		api.DELETE("/timelogs/:id", middleware.CheckPermission("TimelogsListPage", "DELETE"), atthandler.DeleteTimeLog)
		api.GET("/timelogs/schedule-config", middleware.CheckPermission("TimelogsListPage", "VIEW"), atthandler.GetTimelogScheduleConfig)

		api.GET("/leaves/requests", middleware.CheckPermission("LeaveRequestsPage", "VIEW"), atthandler.GetLeaveRequests)
		api.POST("/leaves/requests", middleware.CheckPermission("LeaveRequestsPage", "CREATE"), atthandler.CreateLeaveRequest)
		api.PUT("/leaves/requests/:id", middleware.CheckPermission("LeaveRequestsPage", "EDIT"), atthandler.UpdateLeaveRequest)
		api.DELETE("/leaves/requests/:id", middleware.CheckPermission("LeaveRequestsPage", "DELETE"), atthandler.DeleteLeaveRequest)

		api.GET("/leaves/approved", middleware.CheckPermission("LeaveApprovedPage", "VIEW"), atthandler.GetPendingLeaveRequests)
		api.PUT("/leaves/approve/:id", middleware.CheckPermission("LeaveApprovedPage", "EDIT"), atthandler.ApproveLeaveRequest)

		api.GET("/attendance/config", middleware.CheckPermission("AttendanceSettingsPage", "VIEW"), atthandler.GetAttendanceConfigs)
		api.POST("/attendance/config", middleware.CheckPermission("AttendanceSettingsPage", "EDIT"), atthandler.UpdateAttendanceConfig)

		api.GET("/deduction/summary", middleware.CheckPermission("DeductionSummaryPage", "VIEW"), atthandler.GetAttendanceSummary)
		api.GET("/earning/summary", middleware.CheckPermission("EarningSummaryPage", "VIEW"), atthandler.GetEarningSummary)

		// --- Payroll Processing ---
		api.GET("/payroll/cycles", middleware.CheckPermission("PayrollCyclesPage", "VIEW"), payhandler.GetPayrollCycles)
		api.POST("/payroll/cycles", middleware.CheckPermission("PayrollCyclesPage", "CREATE"), payhandler.CreatePayrollCycle)
		api.PATCH("/payroll/cycles/:id/close", middleware.CheckPermission("PayrollCyclesPage", "EDIT"), payhandler.ClosePayrollCycle)
		api.GET("/payroll/pre-check/:cycle_id", middleware.CheckPermission("RunPayrollPage", "EDIT"), payhandler.PreCheckPayroll)
		api.GET("/payroll/diff/:cycle_id", middleware.CheckPermission("RunPayrollPage", "VIEW"), payhandler.GetPayrollDiff)
		api.POST("/payroll/calculate/:cycle_id", middleware.CheckPermission("RunPayrollPage", "EDIT"), payhandler.CalculatePayroll)
		api.GET("/payroll/transactions/:cycle_id", middleware.CheckPermissionAny([]string{"TransactionsPage", "RunPayrollPage", "PayslipsReportPage", "TaxFormsReportPage"}, "VIEW"), payhandler.GetPayrollTransactions)
		api.GET("/payroll/slip/:transaction_id", middleware.CheckPermissionAny([]string{"TransactionsPage", "RunPayrollPage"}, "VIEW"), payhandler.GetPayrollSlip)
		api.GET("/payroll/my-slips", payhandler.GetMySlips)

		api.GET("/cycle-earnings", middleware.CheckPermission("CycleEarningsPage", "VIEW"), payhandler.GetCycleEarnings)
		api.POST("/cycle-earnings", middleware.CheckPermission("CycleEarningsPage", "CREATE"), payhandler.CreateCycleEarning)
		api.PUT("/cycle-earnings/:id", middleware.CheckPermission("CycleEarningsPage", "EDIT"), payhandler.UpdateCycleEarning)
		api.DELETE("/cycle-earnings/:id", middleware.CheckPermission("CycleEarningsPage", "DELETE"), payhandler.DeleteCycleEarning)

		api.GET("/cycle-deductions", middleware.CheckPermission("CycleDeductionsPage", "VIEW"), payhandler.GetCycleDeductions)
		api.POST("/cycle-deductions", middleware.CheckPermission("CycleDeductionsPage", "CREATE"), payhandler.CreateCycleDeduction)
		api.PUT("/cycle-deductions/:id", middleware.CheckPermission("CycleDeductionsPage", "EDIT"), payhandler.UpdateCycleDeduction)
		api.DELETE("/cycle-deductions/:id", middleware.CheckPermission("CycleDeductionsPage", "DELETE"), payhandler.DeleteCycleDeduction)

		// --- Payroll Config ---
		api.GET("/global", middleware.CheckPermission("PayrollGlobalSettingsPage", "VIEW"), payhandler.GetPayrollConfigs)
		api.POST("/global", middleware.CheckPermission("PayrollGlobalSettingsPage", "EDIT"), payhandler.UpdatePayrollConfig)
		api.GET("/payroll/config/:cycle_id", middleware.CheckPermission("RunPayrollPage", "VIEW"), cfghandler.GetConfigByPayrollCycle)

		api.GET("/earning-types", middleware.CheckPermission("EarningTypesPage", "VIEW"), cfghandler.GetEarningTypes)
		// No CheckPermission gate: handler limits results to caller's accessible
		// companies, and any role assigning employee/cycle earnings needs this dropdown.
		api.GET("/earning-types/options", cfghandler.GetEarningTypeOptions)
		api.POST("/earning-types", middleware.CheckPermission("EarningTypesPage", "CREATE"), cfghandler.CreateEarningType)
		api.PUT("/earning-types/:id", middleware.CheckPermission("EarningTypesPage", "EDIT"), cfghandler.UpdateEarningType)
		api.DELETE("/earning-types/:id", middleware.CheckPermission("EarningTypesPage", "EDIT"), middleware.PreventDeleteIfReferenced("earning_type"), cfghandler.DeleteEarningType)

		api.GET("/deduction-types", middleware.CheckPermission("DeductionTypesPage", "VIEW"), cfghandler.GetDeductionTypes)
		// No CheckPermission gate: handler limits results to caller's accessible
		// companies, and any role assigning employee/cycle deductions needs this dropdown.
		api.GET("/deduction-types/options", cfghandler.GetDeductionTypeOptions)
		api.POST("/deduction-types", middleware.CheckPermission("DeductionTypesPage", "EDIT"), cfghandler.CreateDeductionType)
		api.PUT("/deduction-types/:id", middleware.CheckPermission("DeductionTypesPage", "EDIT"), cfghandler.UpdateDeductionType)
		api.DELETE("/deduction-types/:id", middleware.CheckPermission("DeductionTypesPage", "EDIT"), middleware.PreventDeleteIfReferenced("deduction_type"), cfghandler.DeleteDeductionType)

		api.GET("/master/employment-types", middleware.CheckPermission("EmploymentTypePage", "VIEW"), cfghandler.GetEmploymentTypes)
		// No CheckPermission gate: handler limits results to caller's accessible
		// companies, and any role filling out an employee-linked form needs this dropdown.
		api.GET("/master/employment-types/options", cfghandler.GetEmploymentTypeOptions)
		api.POST("/master/employment-types", middleware.CheckPermission("EmploymentTypePage", "CREATE"), cfghandler.CreateEmploymentType)
		api.PUT("/master/employment-types/:id", middleware.CheckPermission("EmploymentTypePage", "EDIT"), cfghandler.UpdateEmploymentType)
		api.DELETE("/master/employment-types/:id", middleware.CheckPermission("EmploymentTypePage", "DELETE"), middleware.PreventDeleteIfReferenced("employment_type"), cfghandler.DeleteEmploymentType)

		// --- Settings ---
		api.GET("/departments", middleware.CheckPermission("DepartmentSettings", "VIEW"), sethandler.GetDepartments)
		// No CheckPermission gate: handler limits results to caller's accessible
		// companies, and any role filling out an employee-linked form needs this dropdown.
		api.GET("/departments/options", sethandler.GetDepartmentOptions)
		api.POST("/departments", middleware.CheckPermission("DepartmentSettings", "CREATE"), sethandler.CreateDepartment)
		api.PUT("/departments/:id", middleware.CheckPermission("DepartmentSettings", "EDIT"), sethandler.UpdateDepartment)
		api.DELETE("/departments/:id", middleware.CheckPermission("DepartmentSettings", "DELETE"), middleware.PreventDeleteIfReferenced("department"), sethandler.DeleteDepartment)
		api.GET("/department-categories", middleware.CheckPermission("DepartmentSettings", "VIEW"), sethandler.GetDepartmentCategories)
		api.POST("/department-categories", middleware.CheckPermission("DepartmentSettings", "CREATE"), sethandler.CreateDepartmentCategory)

		api.GET("/positions", middleware.CheckPermission("PositionSettings", "VIEW"), sethandler.GetPositions)
		// No CheckPermission gate: handler limits results to caller's accessible
		// companies, and any role filling out an employee-linked form needs this dropdown.
		api.GET("/positions/options", sethandler.GetPositionOptions)
		api.POST("/positions", middleware.CheckPermission("PositionSettings", "CREATE"), sethandler.CreatePosition)
		api.PUT("/positions/:id", middleware.CheckPermission("PositionSettings", "EDIT"), sethandler.UpdatePosition)
		api.DELETE("/positions/:id", middleware.CheckPermission("PositionSettings", "DELETE"), middleware.PreventDeleteIfReferenced("position"), sethandler.DeletePosition)

		api.GET("/roles", middleware.CheckPermission("RolesPage", "VIEW"), sethandler.GetRoles)
		// No CheckPermission gate: handler limits results to caller's accessible
		// companies, and any role filling out an employee-linked form or granting
		// company access needs this dropdown.
		api.GET("/roles/options", sethandler.GetRoleOptions)
		api.POST("/roles", middleware.CheckPermission("RolesPage", "EDIT"), sethandler.CreateRole)
		api.PUT("/roles/:id", middleware.CheckPermission("RolesPage", "EDIT"), sethandler.UpdateRole)
		api.DELETE("/roles/:id", middleware.CheckPermission("RolesPage", "EDIT"), middleware.PreventDeleteIfReferenced("role"), sethandler.DeleteRole)

		api.GET("/permissions/:id", middleware.CheckPermission("PermissionSettings", "VIEW"), sethandler.GetRolesPermissions)
		api.POST("/permissions/:id", middleware.CheckPermission("PermissionSettings", "EDIT"), sethandler.UpdateRolesPermissions)

		api.GET("/employee-company-access", middleware.CheckPermission("EmployeeCompanyAccessPage", "VIEW"), sethandler.GetEmployeeCompanyAccess)
		api.POST("/employee-company-access", middleware.CheckPermission("EmployeeCompanyAccessPage", "CREATE"), sethandler.CreateEmployeeCompanyAccess)
		api.PUT("/employee-company-access/:id", middleware.CheckPermission("EmployeeCompanyAccessPage", "EDIT"), sethandler.UpdateEmployeeCompanyAccess)
		api.DELETE("/employee-company-access/:id", middleware.CheckPermission("EmployeeCompanyAccessPage", "DELETE"), sethandler.DeleteEmployeeCompanyAccess)

		api.GET("/companies/all", middleware.CheckPermission("CompaniesPage", "VIEW"), sethandler.GetAllCompanies)
		// No CheckPermission gate: the handler already limits the result to the
		// caller's accessible companies, and every role needs this dropdown.
		api.GET("/companies/options", sethandler.GetCompanies)
		api.POST("/companies", middleware.CheckPermission("CompaniesPage", "CREATE"), sethandler.CreateCompanies)
		api.PUT("/companies/:id", middleware.CheckPermission("CompaniesPage", "EDIT"), sethandler.UpdateCompanies)
		api.DELETE("/companies/:id", middleware.CheckPermission("CompaniesPage", "DELETE"), middleware.PreventDeleteIfReferenced("companies"), sethandler.DeleteCompanies)

		api.GET("/master/banks", middleware.CheckPermission("MasterBankPage", "VIEW"), sethandler.GetBanks)
		api.POST("/master/banks", middleware.CheckPermission("MasterBankPage", "CREATE"), sethandler.CreateBank)
		api.PUT("/master/banks/:id", middleware.CheckPermission("MasterBankPage", "EDIT"), sethandler.UpdateBank)
		api.DELETE("/master/banks/:id", middleware.CheckPermission("MasterBankPage", "DELETE"), middleware.PreventDeleteIfReferenced("bank"), sethandler.DeleteBank)

		api.GET("/master/holidays", middleware.CheckPermission("HolidayCalendarPage", "VIEW"), sethandler.GetHolidays)
		api.POST("/master/holidays", middleware.CheckPermission("HolidayCalendarPage", "CREATE"), sethandler.CreateHoliday)
		api.PUT("/master/holidays/:id", middleware.CheckPermission("HolidayCalendarPage", "EDIT"), sethandler.UpdateHoliday)
		api.DELETE("/master/holidays/:id", middleware.CheckPermission("HolidayCalendarPage", "DELETE"), middleware.PreventDeleteIfReferenced("holiday"), sethandler.DeleteHoliday)

		api.GET("/master/day-offs", middleware.CheckPermission("DayOffPage", "VIEW"), sethandler.GetDayOffs)
		api.GET("/master/day-offs/companies", middleware.CheckPermission("DayOffPage", "VIEW"), sethandler.GetDayOffCompanies)
		api.POST("/master/day-offs", middleware.CheckPermission("DayOffPage", "CREATE"), sethandler.CreateDayOff)
		api.PUT("/master/day-offs/:id", middleware.CheckPermission("DayOffPage", "EDIT"), sethandler.UpdateDayOff)
		api.DELETE("/master/day-offs/:id", middleware.CheckPermission("DayOffPage", "DELETE"), sethandler.DeleteDayOff)

		api.GET("/master/work-locations", middleware.CheckPermission("WorkLocationPage", "VIEW"), sethandler.GetWorkLocations)
		api.POST("/master/work-locations", middleware.CheckPermission("WorkLocationPage", "CREATE"), sethandler.CreateWorkLocation)
		api.PUT("/master/work-locations/:id", middleware.CheckPermission("WorkLocationPage", "EDIT"), sethandler.UpdateWorkLocation)
		api.DELETE("/master/work-locations/:id", middleware.CheckPermission("WorkLocationPage", "DELETE"), middleware.PreventDeleteIfReferenced("work_location"), sethandler.DeleteWorkLocation)

		api.GET("/master/leave-types", middleware.CheckPermission("MasterLeaveTypePage", "VIEW"), sethandler.GetLeaveTypeSettings)
		// No CheckPermission gate: handler limits results to caller's accessible
		// companies, and any role submitting a leave request needs this dropdown.
		api.GET("/master/leave-types/options", sethandler.GetLeaveTypeOptions)
		api.POST("/master/leave-types", middleware.CheckPermission("MasterLeaveTypePage", "CREATE"), sethandler.CreateLeaveTypeSetting)
		api.PUT("/master/leave-types/:id", middleware.CheckPermission("MasterLeaveTypePage", "EDIT"), sethandler.UpdateLeaveTypeSetting)
		api.DELETE("/master/leave-types/:id", middleware.CheckPermission("MasterLeaveTypePage", "DELETE"), middleware.PreventDeleteIfReferenced("leave_type"), sethandler.DeleteLeaveTypeSetting)

		// --- Reports ---
		api.GET("/payslips/:transaction_id", middleware.CheckPermission("PayslipsReportPage", "VIEW"), rephandler.GetPayslipReport)
		api.GET("/reports/bank-transfer/:cycle_id", middleware.CheckPermission("BankTransferReportPage", "VIEW"), rephandler.GetBankTransferReport)
		api.GET("/reports/tax-forms/pnd1/:cycle_id", middleware.CheckPermission("TaxFormsReportPage", "VIEW"), rephandler.GetTaxFormsPND1)
		api.GET("/reports/tax-forms/50tawi/:transaction_id", middleware.CheckPermission("TaxFormsReportPage", "VIEW"), rephandler.GetTax50Tawi)
		api.GET("/reports/sso/:cycle_id", middleware.CheckPermission("SSOReportPage", "VIEW"), rephandler.GetSSOReport)
		api.GET("/reports/payroll-analysis/:cycle_id", middleware.CheckPermission("PayrollAnalysisPage", "VIEW"), rephandler.GetPayrollAnalysis)
	}
	return r
}

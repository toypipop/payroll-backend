package routes

import (
	"payroll-backend/handlers"
	"payroll-backend/middleware"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 1. CORS Configuration (English Comment)
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type", "Accept", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 2. Public Routes (English Comment)
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", handlers.Login)
	}
	r.GET("/api/v1/modules", handlers.GetAppModules)
	// 3. Protected Routes (English Comment)
	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware())
	{
		// --- Employees Management ---
		api.GET("/employees", middleware.CheckPermission("EmployeesListPage", "VIEW"), handlers.GetEmployees)
		api.POST("/employees", middleware.CheckPermission("EmployeesListPage", "CREATE"), handlers.CreateEmployees)
		api.PUT("/employees/:id", middleware.CheckPermission("EmployeesListPage", "EDIT"), handlers.UpdateEmployees)
		api.DELETE("/employees/:id", middleware.CheckPermission("EmployeesListPage", "DELETE"), handlers.DeleteEmployees)

		api.GET("/salaries", middleware.CheckPermission("SalariesStructurePage", "VIEW"), handlers.GetEmployees)
		api.POST("/salaries", middleware.CheckPermission("SalariesStructurePage", "CREATE"), handlers.UpdateSalariesStructure)

		// --- Master Data (Department & Position) ---
		api.GET("/departments", middleware.CheckPermission("DepartmentSettings", "VIEW"), handlers.GetDepartments)
		api.POST("/departments", middleware.CheckPermission("DepartmentSettings", "CREATE"), handlers.CreateDepartment)
		api.PUT("/departments/:id", middleware.CheckPermission("DepartmentSettings", "EDIT"), handlers.UpdateDepartment)
		api.DELETE("/departments/:id", middleware.CheckPermission("DepartmentSettings", "DELETE"), handlers.DeleteDepartment)

		api.GET("/positions", middleware.CheckPermission("PositionSettings", "VIEW"), handlers.GetPositions)
		api.POST("/positions", middleware.CheckPermission("PositionSettings", "CREATE"), handlers.CreatePosition)
		api.PUT("/positions/:id", middleware.CheckPermission("PositionSettings", "EDIT"), handlers.UpdatePosition)
		api.DELETE("/positions/:id", middleware.CheckPermission("PositionSettings", "DELETE"), handlers.DeletePosition)

		// --- Roles Management ---
		api.GET("/roles", middleware.CheckPermission("RolesPage", "VIEW"), handlers.GetRoles)
		api.POST("/roles", middleware.CheckPermission("RolesPage", "EDIT"), handlers.CreateRole)
		api.PUT("/roles/:id", middleware.CheckPermission("RolesPage", "EDIT"), handlers.UpdateRole)
		api.DELETE("/roles/:id", middleware.CheckPermission("RolesPage", "EDIT"), handlers.DeleteRole)

		// --- Permission Management ---
		api.GET("/permissions/:id", middleware.CheckPermission("PermissionSettings", "VIEW"), handlers.GetRolesPermissions)
		api.POST("/permissions/:id", middleware.CheckPermission("PermissionSettings", "EDIT"), handlers.UpdateRolesPermissions)

		// --- Companies Profile Management ---
		api.GET("/companies/all", middleware.CheckPermission("CompaniesPage", "VIEW"), handlers.GetAllCompanies)
		api.GET("/companies/options", middleware.CheckPermission("CompaniesPage", "VIEW"), handlers.GetCompanies)
		api.POST("/companies", middleware.CheckPermission("CompaniesPage", "CREATE"), handlers.CreateCompanies)
		api.PUT("/companies/:id", middleware.CheckPermission("CompaniesPage", "EDIT"), handlers.UpdateCompanies)
		api.DELETE("/companies/:id", middleware.CheckPermission("CompaniesPage", "DELETE"), handlers.DeleteCompanies)

		// --- Time Logs Management ---
		api.GET("/timelogs", middleware.CheckPermission("TimelogsListPage", "VIEW"), handlers.GetTimeLogs)
		api.POST("/timelogs", middleware.CheckPermission("TimelogsListPage", "CREATE"), handlers.CreateTimeLog)
		api.PUT("/timelogs/:id", middleware.CheckPermission("TimelogsListPage", "EDIT"), handlers.UpdateTimeLog)
		api.DELETE("/timelogs/:id", middleware.CheckPermission("TimelogsListPage", "DELETE"), handlers.DeleteTimeLog)

		api.GET("/leaves/requests", middleware.CheckPermission("LeaveRequestsPage", "VIEW"), handlers.GetLeaveRequests)
		api.POST("/leaves/requests", middleware.CheckPermission("LeaveRequestsPage", "CREATE"), handlers.CreateLeaveRequest)
		api.PUT("/leaves/requests/:id", middleware.CheckPermission("LeaveRequestsPage", "EDIT"), handlers.UpdateLeaveRequest)
		api.DELETE("/leaves/requests/:id", middleware.CheckPermission("LeaveRequestsPage", "DELETE"), handlers.DeleteLeaveRequest)

		api.GET("/leaves/approved", middleware.CheckPermission("LeaveApprovedPage", "VIEW"), handlers.GetPendingLeaveRequests)
		api.PUT("/leaves/approve/:id", middleware.CheckPermission("LeaveApprovedPage", "EDIT"), handlers.ApproveLeaveRequest)

		// --- Payroll Global Config ---
		api.GET("/global", middleware.CheckPermission("PayrollGlobalSettingsPage", "VIEW"), handlers.GetPayrollConfigs)
		api.POST("/global", middleware.CheckPermission("PayrollGlobalSettingsPage", "EDIT"), handlers.UpdatePayrollConfig)

		// --- Earning Types CRUD (English Comment) ---
		api.GET("/earning-types", middleware.CheckPermission("EarningTypesPage", "VIEW"), handlers.GetEarningTypes)
		api.POST("/earning-types", middleware.CheckPermission("EarningTypesPage", "CREATE"), handlers.CreateEarningType)
		api.PUT("/earning-types/:id", middleware.CheckPermission("EarningTypesPage", "EDIT"), handlers.UpdateEarningType)
		api.DELETE("/earning-types/:id", middleware.CheckPermission("EarningTypesPage", "EDIT"), handlers.DeleteEarningType)

		// --- Deduction Types CRUD (English Comment) ---
		api.GET("/deduction-types", middleware.CheckPermission("DeductionTypesPage", "VIEW"), handlers.GetDeductionTypes)
		api.POST("/deduction-types", middleware.CheckPermission("DeductionTypesPage", "EDIT"), handlers.CreateDeductionType)
		api.PUT("/deduction-types/:id", middleware.CheckPermission("DeductionTypesPage", "EDIT"), handlers.UpdateDeductionType)
		api.DELETE("/deduction-types/:id", middleware.CheckPermission("DeductionTypesPage", "EDIT"), handlers.DeleteDeductionType)

		// --- Payroll Cycle Variable Earnings ---
		api.GET("/cycle-earnings", middleware.CheckPermission("RunPayrollPage", "VIEW"), handlers.GetCycleEarnings)
		api.POST("/cycle-earnings", middleware.CheckPermission("RunPayrollPage", "CREATE"), handlers.CreateCycleEarning)
		api.PUT("/cycle-earnings/:id", middleware.CheckPermission("RunPayrollPage", "EDIT"), handlers.UpdateCycleEarning)
		api.DELETE("/cycle-earnings/:id", middleware.CheckPermission("RunPayrollPage", "DELETE"), handlers.DeleteCycleEarning)

		// --- Payroll Cycle Variable Deductions ---
		api.GET("/cycle-deductions", middleware.CheckPermission("RunPayrollPage", "VIEW"), handlers.GetCycleDeductions)
		api.POST("/cycle-deductions", middleware.CheckPermission("RunPayrollPage", "CREATE"), handlers.CreateCycleDeduction)
		api.PUT("/cycle-deductions/:id", middleware.CheckPermission("RunPayrollPage", "EDIT"), handlers.UpdateCycleDeduction)
		api.DELETE("/cycle-deductions/:id", middleware.CheckPermission("RunPayrollPage", "DELETE"), handlers.DeleteCycleDeduction)

		// --- Employee Earning Assignments ---
		api.GET("/employee-earnings", middleware.CheckPermission("EmployeesListPage", "VIEW"), handlers.GetEmployeeEarnings)
		api.POST("/employee-earnings", middleware.CheckPermission("EmployeesListPage", "CREATE"), handlers.CreateEmployeeEarning)
		api.PUT("/employee-earnings/:id", middleware.CheckPermission("EmployeesListPage", "EDIT"), handlers.UpdateEmployeeEarning)
		api.DELETE("/employee-earnings/:id", middleware.CheckPermission("EmployeesListPage", "DELETE"), handlers.DeleteEmployeeEarning)

		// --- Employee Deduction Assignments ---
		api.GET("/employee-deductions", middleware.CheckPermission("EmployeesListPage", "VIEW"), handlers.GetEmployeeDeductions)
		api.POST("/employee-deductions", middleware.CheckPermission("EmployeesListPage", "CREATE"), handlers.CreateEmployeeDeduction)
		api.PUT("/employee-deductions/:id", middleware.CheckPermission("EmployeesListPage", "EDIT"), handlers.UpdateEmployeeDeduction)
		api.DELETE("/employee-deductions/:id", middleware.CheckPermission("EmployeesListPage", "DELETE"), handlers.DeleteEmployeeDeduction)

		// --- Attendance Config ---
		api.GET("/attendance/config", middleware.CheckPermission("AttendanceSettingsPage", "VIEW"), handlers.GetAttendanceConfigs)
		api.POST("/attendance/config", middleware.CheckPermission("AttendanceSettingsPage", "EDIT"), handlers.UpdateAttendanceConfig)

		// --- Attendance Summary ---
		api.GET("/attendance/summary", middleware.CheckPermission("AttendanceSummaryPage", "VIEW"), handlers.GetAttendanceSummary)

		// --- Payroll Core Engine ---
		api.GET("/payroll/cycles", middleware.CheckPermission("PayrollCyclesPage", "VIEW"), handlers.GetPayrollCycles)
		api.POST("/payroll/cycles", middleware.CheckPermission("PayrollCyclesPage", "CREATE"), handlers.CreatePayrollCycle)
		api.PATCH("/payroll/cycles/:id/close", middleware.CheckPermission("PayrollCyclesPage", "EDIT"), handlers.ClosePayrollCycle)
		api.POST("/payroll/calculate/:cycle_id", middleware.CheckPermission("RunPayrollPage", "EDIT"), handlers.CalculatePayroll)
		api.GET("/payroll/transactions/:cycle_id", middleware.CheckPermission("RunPayrollPage", "VIEW"), handlers.GetPayrollTransactions)
		api.GET("/payroll/slip/:transaction_id", middleware.CheckPermission("RunPayrollPage", "VIEW"), handlers.GetPayrollSlip)

		// --- Reports ---
		api.GET("/payslips/:transaction_id", middleware.CheckPermission("PayslipsReportPage", "VIEW"), handlers.GetPayrollSlip)
	}
	return r
}

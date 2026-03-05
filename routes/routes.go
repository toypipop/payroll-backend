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

	// 3. Protected Routes (English Comment)
	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware())
	{
		api.GET("/modules", handlers.GetAppModules)

		// --- Employees Management ---
		// Use ModuleEmployees or EmployeesListPage depending on your permission logic
		api.GET("/employees", middleware.CheckPermission("EmployeesListPage", "VIEW"), handlers.GetEmployees)
		api.POST("/employees", middleware.CheckPermission("EmployeesListPage", "CREATE"), handlers.CreateEmployees)
		api.PUT("/employees/:id", middleware.CheckPermission("EmployeesListPage", "EDIT"), handlers.UpdateEmployees)
		api.DELETE("/employees/:id", middleware.CheckPermission("EmployeesListPage", "DELETE"), handlers.DeleteEmployees)

		// Salaries usually falls under Employee Management permissions
		api.GET("/salaries", middleware.CheckPermission("SalariesStructurePage", "VIEW"), handlers.GetEmployees)
		api.POST("/salaries", middleware.CheckPermission("SalariesStructurePage", "EDIT"), handlers.UpdateSalariesStructure)

		// --- Master Data (Department & Position) ---
		// Use ModuleSettings as the parent key for all setting-related sub-menus
		api.GET("/departments", middleware.CheckPermission("DepartmentSettings", "VIEW"), handlers.GetDepartments)
		api.POST("/departments", middleware.CheckPermission("DepartmentSettings", "CREATE"), handlers.CreateDepartment)
		api.PUT("/departments/:id", middleware.CheckPermission("DepartmentSettings", "EDIT"), handlers.UpdateDepartment)
		api.DELETE("/departments/:id", middleware.CheckPermission("DepartmentSettings", "DELETE"), handlers.DeleteDepartment)

		api.GET("/positions", middleware.CheckPermission("PositionSettings", "VIEW"), handlers.GetPositions)
		api.POST("/positions", middleware.CheckPermission("PositionSettings", "CREATE"), handlers.CreatePosition)
		api.PUT("/positions/:id", middleware.CheckPermission("PositionSettings", "EDIT"), handlers.UpdatePosition)
		api.DELETE("/positions/:id", middleware.CheckPermission("PositionSettings", "DELETE"), handlers.DeletePosition)

		// --- Roles & Permission Management ---
		api.GET("/roles", middleware.CheckPermission("PermissionSettings", "VIEW"), handlers.GetRoles)
		api.GET("/roles/:id/permissions", middleware.CheckPermission("PermissionSettings", "VIEW"), handlers.GetRolesPermissions)
		api.POST("/roles/:id/permissions", middleware.CheckPermission("PermissionSettings", "EDIT"), handlers.UpdateRolesPermissions)

		// --- Companies Profile Management ---
		// Using CompaniesPage or a dedicated ModuleKey if defined
		api.GET("/companies/all", middleware.CheckPermission("CompaniesPage", "VIEW"), handlers.GetAllCompanies)
		api.GET("/companies/options", middleware.CheckPermission("CompaniesPage", "VIEW"), handlers.GetCompanies)
		api.POST("/companies", middleware.CheckPermission("CompaniesPage", "CREATE"), handlers.CreateCompanies)
		api.PUT("/companies/:id", middleware.CheckPermission("CompaniesPage", "EDIT"), handlers.UpdateCompanies)
		api.DELETE("/companies/:id", middleware.CheckPermission("CompaniesPage", "DELETE"), handlers.DeleteCompanies)

		// --- Time Logs Management ---
		// Use ModuleAttendance as defined in your Level 1 categories
		api.GET("/timelogs", middleware.CheckPermission("TimelogsListPage", "VIEW"), handlers.GetTimeLogs)
		api.POST("/timelogs", middleware.CheckPermission("TimelogsListPage", "CREATE"), handlers.CreateTimeLog)
		api.PUT("/timelogs/:id", middleware.CheckPermission("TimelogsListPage", "EDIT"), handlers.UpdateTimeLog)
		api.DELETE("/timelogs/:id", middleware.CheckPermission("TimelogsListPage", "DELETE"), handlers.DeleteTimeLog)

		// Use ModuleAttendance key for unified attendance-related permissions (English Comment)
		api.GET("/leaves/requests", middleware.CheckPermission("LeaveRequestsPage", "VIEW"), handlers.GetLeaveRequests)
		api.POST("/leaves/requests", middleware.CheckPermission("LeaveRequestsPage", "CREATE"), handlers.CreateLeaveRequest)
		api.PUT("/leaves/requests/:id", middleware.CheckPermission("LeaveRequestsPage", "EDIT"), handlers.UpdateLeaveRequest)
		api.DELETE("/leaves/requests/:id", middleware.CheckPermission("LeaveRequestsPage", "DELETE"), handlers.DeleteLeaveRequest)

		api.GET("/leaves/approved", middleware.CheckPermission("LeaveApprovedPage", "VIEW"), handlers.GetPendingLeaveRequests)
		api.PUT("/leaves/approve/:id", middleware.CheckPermission("LeaveApprovedPage", "EDIT"), handlers.ApproveLeaveRequest)

		api.GET("/global", middleware.CheckPermission("PayrollGlobalSettingsPage", "VIEW"), handlers.GetPayrollConfigs)
		api.POST("/global", middleware.CheckPermission("PayrollGlobalSettingsPage", "EDIT"), handlers.UpdatePayrollConfig)

	}
	return r
}

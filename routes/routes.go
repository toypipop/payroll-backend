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
		api.GET("/employees", middleware.CheckPermission("ModuleEmployees", "VIEW"), handlers.GetEmployees)
		api.POST("/employees", middleware.CheckPermission("ModuleEmployees", "CREATE"), handlers.CreateEmployees)
		api.PUT("/employees/:id", middleware.CheckPermission("ModuleEmployees", "EDIT"), handlers.UpdateEmployees)
		api.DELETE("/employees/:id", middleware.CheckPermission("ModuleEmployees", "DELETE"), handlers.DeleteEmployees)

		// Salaries usually falls under Employee Management permissions
		api.POST("/salaries", middleware.CheckPermission("ModuleEmployees", "EDIT"), handlers.UpdateSalariesStructure)

		// --- Master Data (Department & Position) ---
		// Use ModuleSettings as the parent key for all setting-related sub-menus
		api.GET("/departments", middleware.CheckPermission("ModuleSettings", "VIEW"), handlers.GetDepartments)
		api.POST("/departments", middleware.CheckPermission("ModuleSettings", "CREATE"), handlers.CreateDepartment)
		api.PUT("/departments/:id", middleware.CheckPermission("ModuleSettings", "EDIT"), handlers.UpdateDepartment)
		api.DELETE("/departments/:id", middleware.CheckPermission("ModuleSettings", "DELETE"), handlers.DeleteDepartment)

		api.GET("/positions", middleware.CheckPermission("ModuleSettings", "VIEW"), handlers.GetPositions)
		api.POST("/positions", middleware.CheckPermission("ModuleSettings", "CREATE"), handlers.CreatePosition)
		api.PUT("/positions/:id", middleware.CheckPermission("ModuleSettings", "EDIT"), handlers.UpdatePosition)
		api.DELETE("/positions/:id", middleware.CheckPermission("ModuleSettings", "DELETE"), handlers.DeletePosition)

		// --- Roles & Permission Management ---
		api.GET("/roles", middleware.CheckPermission("ModuleSettings", "VIEW"), handlers.GetRoles)
		api.GET("/roles/:id/permissions", middleware.CheckPermission("ModuleSettings", "VIEW"), handlers.GetRolesPermissions)
		api.POST("/roles/:id/permissions", middleware.CheckPermission("ModuleSettings", "EDIT"), handlers.UpdateRolesPermissions)

		// --- Companies Profile Management ---
		// Using CompaniesPage or a dedicated ModuleKey if defined
		api.GET("/companies/all", middleware.CheckPermission("CompaniesPage", "VIEW"), handlers.GetAllCompanies)
		api.GET("/companies/options", middleware.CheckPermission("CompaniesPage", "VIEW"), handlers.GetCompanies)
		api.POST("/companies", middleware.CheckPermission("CompaniesPage", "CREATE"), handlers.CreateCompanies)
		api.PUT("/companies/:id", middleware.CheckPermission("CompaniesPage", "EDIT"), handlers.UpdateCompanies)
		api.DELETE("/companies/:id", middleware.CheckPermission("CompaniesPage", "DELETE"), handlers.DeleteCompanies)

		// --- Time Logs Management ---
		// Use ModuleAttendance as defined in your Level 1 categories
		api.GET("/timelogs", middleware.CheckPermission("ModuleAttendance", "VIEW"), handlers.GetTimeLogs)
		api.POST("/timelogs", middleware.CheckPermission("ModuleAttendance", "CREATE"), handlers.CreateTimeLog)
		api.PUT("/timelogs/:id", middleware.CheckPermission("ModuleAttendance", "EDIT"), handlers.UpdateTimeLog)
		api.DELETE("/timelogs/:id", middleware.CheckPermission("ModuleAttendance", "DELETE"), handlers.DeleteTimeLog)
	}
	return r
}

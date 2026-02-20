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
		// --- Employees Management (CRUD with specific permissions) ---
		api.GET("/employees", middleware.CheckPermission("EMPLOYEES", "VIEW"), handlers.GetEmployees)
		api.POST("/employees", middleware.CheckPermission("EMPLOYEES", "CREATE"), handlers.CreateEmployees)
		api.PUT("/employees/:id", middleware.CheckPermission("EMPLOYEES", "EDIT"), handlers.UpdateEmployees)
		api.DELETE("/employees/:id", middleware.CheckPermission("EMPLOYEES", "DELETE"), handlers.DeleteEmployees)

		// --- Master Data (Department & Position Management) ---
		api.GET("/departments", middleware.CheckPermission("SETTINGS", "VIEW"), handlers.GetDepartments)
		api.POST("/departments", middleware.CheckPermission("SETTINGS", "CREATE"), handlers.CreateDepartment)
		api.PUT("/departments/:id", middleware.CheckPermission("SETTINGS", "EDIT"), handlers.UpdateDepartment)
		api.DELETE("/departments/:id", middleware.CheckPermission("SETTINGS", "DELETE"), handlers.DeleteDepartment)

		api.GET("/positions", middleware.CheckPermission("SETTINGS", "VIEW"), handlers.GetPositions)
		// api.POST("/positions", middleware.CheckPermission("POSITION", "CREATE"), handlers.CreatePosition)

		// --- Roles & Permission Management (High security area) ---
		api.GET("/roles", middleware.CheckPermission("SETTINGS", "VIEW"), handlers.GetRoles)
		api.GET("/roles/:id/permissions", middleware.CheckPermission("SETTINGS", "VIEW"), handlers.GetRolesPermissions)
		api.POST("/roles/:id/permissions", middleware.CheckPermission("SETTINGS", "EDIT"), handlers.UpdateRolesPermissions)

		// --- Companies Profile Management ---
		api.GET("/companies/all", middleware.CheckPermission("COMPANIES", "VIEW"), handlers.GetAllCompanies)
		api.GET("/companies/options", middleware.CheckPermission("COMPANIES", "VIEW"), handlers.GetCompanies)
		api.POST("/companies", middleware.CheckPermission("COMPANIES", "CREATE"), handlers.CreateCompanies)
		api.PUT("/companies/:id", middleware.CheckPermission("COMPANIES", "EDIT"), handlers.UpdateCompanies)
		api.DELETE("/companies/:id", middleware.CheckPermission("COMPANIES", "DELETE"), handlers.DeleteCompanies)

		// routes/router.go
		api.POST("/salaries", middleware.CheckPermission("EMPLOYEES", "EDIT"), handlers.UpdateSalariesStructure)
	}
	return r
}

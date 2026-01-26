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
		// --- Employee Management (CRUD with specific permissions) ---
		api.GET("/employees", middleware.CheckPermission("EMPLOYEE", "VIEW"), handlers.GetEmployees)
		api.POST("/employees", middleware.CheckPermission("EMPLOYEE", "CREATE"), handlers.CreateEmployee)
		api.PUT("/employees/:id", middleware.CheckPermission("EMPLOYEE", "EDIT"), handlers.UpdateEmployee)
		api.DELETE("/employees/:id", middleware.CheckPermission("EMPLOYEE", "DELETE"), handlers.DeleteEmployee)

		// --- Master Data (Department & Position Management) ---
		api.GET("/departments", middleware.CheckPermission("DEPARTMENT", "VIEW"), handlers.GetDepartments)
		api.POST("/departments", middleware.CheckPermission("DEPARTMENT", "CREATE"), handlers.CreateDepartment)

		api.GET("/positions", middleware.CheckPermission("POSITION", "VIEW"), handlers.GetPositions)
		// api.POST("/positions", middleware.CheckPermission("POSITION", "CREATE"), handlers.CreatePosition)

		// --- Role & Permission Management (High security area) ---
		api.GET("/roles", middleware.CheckPermission("SETTING", "VIEW"), handlers.GetRoles)
		api.GET("/roles/:id/permissions", middleware.CheckPermission("SETTING", "VIEW"), handlers.GetRolePermissions)
		api.POST("/roles/:id/permissions", middleware.CheckPermission("SETTING", "EDIT"), handlers.UpdateRolePermissions)

		// --- Company Profile Management ---
		api.GET("/company", middleware.CheckPermission("SETTING", "VIEW"), handlers.GetCompany)
		api.PUT("/company/:id", middleware.CheckPermission("SETTING", "EDIT"), handlers.UpdateCompany)
	}

	return r
}

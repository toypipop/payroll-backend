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
	api.Use(middleware.AuthMiddleware()) // Must be before CheckPermission
	{
		// --- Employee Management ---
		api.GET("/employees", middleware.CheckPermission("EMPLOYEE", false), handlers.GetEmployees)
		api.POST("/employees", middleware.CheckPermission("EMPLOYEE", true), handlers.CreateEmployee)
		api.PUT("/employees/:id", middleware.CheckPermission("EMPLOYEE", true), handlers.UpdateEmployee)
		api.DELETE("/employees/:id", middleware.CheckPermission("EMPLOYEE", true), handlers.DeleteEmployee)

		// --- Master Data (View only usually) ---
		api.GET("/departments", middleware.CheckPermission("DEPARTMENT", false), handlers.GetDepartments)
		api.GET("/positions", middleware.CheckPermission("POSITION", false), handlers.GetPositions)
		api.GET("/roles", handlers.GetRoles)

		// --- Company Settings ---
		api.GET("/company", middleware.CheckPermission("SETTING", false), handlers.GetCompany)
		api.POST("/company", middleware.CheckPermission("SETTING", true), handlers.CreateCompany)
		api.PUT("/company/:id", middleware.CheckPermission("SETTING", true), handlers.UpdateCompany)
		api.DELETE("/company/:id", middleware.CheckPermission("SETTING", true), handlers.DeleteCompany)

	}

	return r
}

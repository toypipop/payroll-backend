package routes

import (
	"payroll-backend/handlers"
	"payroll-backend/middleware" // Import middleware
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// Add CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"}, // React port
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Public routes
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", handlers.Login)
	}

	// Protected routes (Must have token)
	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware()) // Use middleware for this group
	{
		api.GET("/employees", handlers.GetEmployees)
		api.POST("/employees", handlers.CreateEmployee)
		api.PUT("/employees/:id", handlers.UpdateEmployee)
		api.DELETE("/employees/:id", handlers.DeleteEmployee)

		api.GET("/departments", handlers.GetDepartments)
		api.GET("/positions", handlers.GetPositions)

		api.GET("/company", handlers.GetCompany)
		api.POST("/company", handlers.CreateCompany)
		api.PUT("/company/:id", handlers.UpdateCompany)
		api.DELETE("/company/:id", handlers.DeleteCompany)
	}

	return r
}

package routes

import (
	"payroll-backend/handlers"
	"payroll-backend/middleware" // Import middleware

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

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
		// Other protected routes go here
	}

	return r
}

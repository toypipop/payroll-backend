package handlers

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Secret key for signing JWT (In production, use environment variables)
var jwtSecret = []byte("your_super_secret_key_2026")

// Login processes employee authentication
func Login(c *gin.Context) {
	var input models.LoginRequest
	var employee models.Employee

	// 1. Validate input format
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return
	}

	// 2. Find employee by email
	if err := config.DB.Where("email = ?", input.Email).First(&employee).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// 3. Compare hashed password
	err := bcrypt.CompareHashAndPassword([]byte(employee.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// 4. Create JWT Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": employee.ID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(), // Token expires in 24 hours
		"iat":     time.Now().Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	// 5. Send response
	c.JSON(http.StatusOK, models.LoginResponse{
		Token:    tokenString,
		FullName: employee.FirstName + " " + employee.LastName,
		Role:     "ADMIN", // Temporary; link to roles table later
	})
}

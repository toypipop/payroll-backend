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

var jwtSecret = []byte("your_super_secret_key_2026")

func GetJWTSecret() []byte {
	return jwtSecret
}

/* --- Custom JWT Claims structure (English Comment) --- */
type TokenClaims struct {
	UserID    uint `json:"user_id"`
	CompanyID uint `json:"company_id"` // Crucial for Multi-tenancy logic
	RoleID    uint `json:"role_id"`    // For RBAC logic
	jwt.RegisteredClaims
}

func Login(c *gin.Context) {
	var input models.LoginRequest
	var employee models.Employee

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return
	}

	// Find employee and preload their Role if necessary (English Comment)
	if err := config.DB.Where("email = ?", input.Email).First(&employee).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(employee.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	/* --- 4. Create JWT Token with Multi-tenant Claims (English Comment) --- */
	claims := TokenClaims{
		UserID:    employee.ID,
		CompanyID: employee.CompanyID, // Link token to company
		// Assuming your employee model will have RoleID field (English Comment)
		// RoleID: employee.RoleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{
		Token:    tokenString,
		FullName: employee.FirstName + " " + employee.LastName,
		Role:     "ADMIN", // Logic to fetch role name can be added here
	})
}

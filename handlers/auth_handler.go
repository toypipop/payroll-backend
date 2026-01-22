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

type TokenClaims struct {
	UserID       uint `json:"user_id"`
	CompanyID    uint `json:"company_id"`
	RoleID       uint `json:"role_id"`
	IsSuperAdmin bool `json:"is_super_admin"`
	jwt.RegisteredClaims
}

func Login(c *gin.Context) {
	var input models.LoginRequest
	var employee models.Employee

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return
	}

	// 1. Find employee and Preload Role to get RoleName later (English Comment)
	if err := config.DB.Preload("Role").Where("email = ?", input.Email).First(&employee).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// 2. Check Password (English Comment)
	err := bcrypt.CompareHashAndPassword([]byte(employee.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// 3. Create Token with RoleID (English Comment)
	claims := TokenClaims{
		UserID:       employee.ID,
		CompanyID:    employee.CompanyID,
		RoleID:       employee.RoleID,
		IsSuperAdmin: employee.Role.IsSuperAdmin,
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

	// 4. Send Response (English Comment)
	c.JSON(http.StatusOK, models.LoginResponse{
		Token:    tokenString,
		FullName: employee.FirstName + " " + employee.LastName,
		Role:     employee.Role.RoleName, // Return actual role name to frontend
	})
}

package auth

import (
	"errors"
	"net/http"
	"os"
	"payroll-backend/config"
	"payroll-backend/models"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func GetJWTSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("JWT secret is not configured")
	}
	return []byte(secret), nil
}

type TokenClaims struct {
	UserID       uint `json:"user_id"`
	CompaniesID  uint `json:"companies_id"`
	RolesID      uint `json:"roles_id"`
	IsSuperAdmin bool `json:"is_super_admin"`
	jwt.RegisteredClaims
}

func Login(c *gin.Context) {
	var input models.LoginRequest
	var Employees models.Employees

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return
	}

	// Find and load employee with roles
	if err := config.DB.Preload("Roles").Where("email = ?", input.Email).First(&Employees).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email"})
		return
	}

	// Verify password
	err := bcrypt.CompareHashAndPassword([]byte(Employees.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	jwtSecret, err := GetJWTSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication service is not configured"})
		return
	}

	// Create JWT token with claims
	claims := TokenClaims{
		UserID:       Employees.ID,
		CompaniesID:  Employees.CompaniesID,
		RolesID:      Employees.RolesID,
		IsSuperAdmin: Employees.Roles.IsSuperAdmin,
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

	// Return login response
	c.JSON(http.StatusOK, models.LoginResponse{
		Token:    tokenString,
		ID:       Employees.ID,
		FullName: Employees.FirstName + " " + Employees.LastName,
		Roles:    Employees.Roles.RolesName,
	})
}

func GetAppModules(c *gin.Context) {
	// AppModule is global master data
	var modules []models.AppModule

	// Fetch active modules sorted
	if err := config.DB.
		Where("is_active = ?", true).
		Order("`order` ASC"). // Use backticks because 'order' is a reserved word in SQL
		Find(&modules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch app modules"})
		return
	}
	c.JSON(http.StatusOK, modules)
}

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
	CompaniesID    uint `json:"companies_id"`
	RolesID       uint `json:"roles_id"`
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

	// 1. Find Employees and Preload Roles to get RolesName later (English Comment)
	if err := config.DB.Preload("Roles").Where("email = ?", input.Email).First(&Employees).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email"})
		return
	}

	// 2. Check Password (English Comment)
	err := bcrypt.CompareHashAndPassword([]byte(Employees.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	// 3. Create Token with RolesID (English Comment)
	claims := TokenClaims{
		UserID:       Employees.ID,
		CompaniesID:    Employees.CompaniesID,
		RolesID:       Employees.RolesID,
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

	// 4. Send Response (English Comment)
	c.JSON(http.StatusOK, models.LoginResponse{
		Token:    tokenString,
		FullName: Employees.FirstName + " " + Employees.LastName,
		Roles:     Employees.Roles.RolesName, // Return actual roles name to frontend
	})
}

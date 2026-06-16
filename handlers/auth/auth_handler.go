package auth

import (
	"errors"
	"net/http"
	"os"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/clause"
)

const (
	authCookieName = "auth_token"
	authTokenTTL   = 24 * time.Hour
)

func GetJWTSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("JWT secret is not configured")
	}
	return []byte(secret), nil
}

type TokenClaims struct {
	UserID      uint `json:"user_id"`
	CompaniesID uint `json:"companies_id"`
	RolesID     uint `json:"roles_id"`
	jwt.RegisteredClaims
}

func isProduction() bool {
	return strings.EqualFold(os.Getenv("APP_ENV"), "production") ||
		strings.EqualFold(os.Getenv("GO_ENV"), "production") ||
		strings.EqualFold(os.Getenv("GIN_MODE"), "release")
}

func envBool(key string) (bool, bool) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false, false
	}

	switch strings.ToLower(value) {
	case "1", "true", "t", "yes", "y", "on":
		return true, true
	case "0", "false", "f", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func cookieSameSite() http.SameSite {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("COOKIE_SAMESITE"))) {
	case "none":
		return http.SameSiteNoneMode
	case "strict":
		return http.SameSiteStrictMode
	case "lax":
		return http.SameSiteLaxMode
	}

	if isProduction() {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func cookieSecure() bool {
	if value, ok := envBool("COOKIE_SECURE"); ok {
		return value
	}
	return isProduction()
}

func SetAuthCookie(c *gin.Context, tokenString string) {
	c.SetSameSite(cookieSameSite())
	c.SetCookie(
		authCookieName,
		tokenString,
		int(authTokenTTL.Seconds()),
		"/",
		"",
		cookieSecure(),
		true,
	)
}

func ClearAuthCookie(c *gin.Context) {
	c.SetSameSite(cookieSameSite())
	c.SetCookie(
		authCookieName,
		"",
		-1,
		"/",
		"",
		cookieSecure(),
		true,
	)
}

func GetAuthCookieName() string {
	return authCookieName
}

func buildLoginResponse(employee models.Employees) models.LoginResponse {
	companyID := repository.DefaultCompanyID(employee)
	companyIDs := repository.AccessibleCompanyIDs(employee.ID)
	if len(companyIDs) == 0 && companyID != 0 {
		companyIDs = []uint{companyID}
	}

	return models.LoginResponse{
		ID:                   employee.ID,
		FullName:             employee.FirstName + " " + employee.LastName,
		Roles:                repository.RoleNames(employee.ID),
		CompaniesID:          companyID,
		AccessibleCompanyIDs: companyIDs,
	}
}

func Login(c *gin.Context) {
	var input models.LoginRequest
	var Employees models.Employees

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return
	}

	if err := config.DB.Where("email = ?", input.Email).First(&Employees).Error; err != nil {
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
	defaultCompanyID, defaultRoleID := repository.DefaultCompanyAccess(Employees)
	claims := TokenClaims{
		UserID:      Employees.ID,
		CompaniesID: defaultCompanyID,
		RolesID:     defaultRoleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(authTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	SetAuthCookie(c, tokenString)
	c.JSON(http.StatusOK, buildLoginResponse(Employees))
}

func Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var employee models.Employees
	if err := config.DB.First(&employee, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	c.JSON(http.StatusOK, buildLoginResponse(employee))
}

func Logout(c *gin.Context) {
	ClearAuthCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

func GetAppModules(c *gin.Context) {
	// AppModule is global master data
	var modules []models.AppModule

	// Fetch active modules sorted
	if err := config.DB.
		Where("is_active = ?", true).
		// Let GORM quote the reserved column name for PostgreSQL instead of hand-writing SQL.
		Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}}).
		Find(&modules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch app modules"})
		return
	}
	c.JSON(http.StatusOK, modules)
}

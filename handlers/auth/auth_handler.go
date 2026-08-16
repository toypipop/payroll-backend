package auth

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	authCookieName = "auth_token"
	authTokenTTL   = 24 * time.Hour
)

// authService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func authService() *service.AuthService {
	return service.NewAuthService(repository.NewAuthRepository())
}

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

// issueToken signs a token for an authenticated employee. Signing and the cookie stay
// in the handler because they are transport, not rules.
func issueToken(subject service.TokenSubject) (string, error) {
	secret, err := GetJWTSecret()
	if err != nil {
		return "", err
	}

	claims := TokenClaims{
		UserID:      subject.UserID,
		CompaniesID: subject.CompaniesID,
		RolesID:     subject.RolesID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(authTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func Login(c *gin.Context) {
	var input models.LoginRequest
	if !httpx.BindJSON(c, &input) {
		return
	}

	employee, err := authService().Authenticate(input.Email, input.Password)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	secret, err := GetJWTSecret()
	if err != nil || len(secret) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication service is not configured"})
		return
	}

	tokenString, err := issueToken(authService().Subject(employee))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	SetAuthCookie(c, tokenString)
	c.JSON(http.StatusOK, authService().Identity(employee))
}

func Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	identity, err := authService().IdentityByID(id)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, identity)
}

func Logout(c *gin.Context) {
	ClearAuthCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

// GetAppModules returns every active module, unfiltered. Permission Settings needs the
// whole tree regardless of the admin's own access.
func GetAppModules(c *gin.Context) {
	modules, err := authService().AllModules()
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, modules)
}

// GetMyAppModules returns the same active module list as GetAppModules, but
// filtered to what the requesting user can actually VIEW: a leaf module is
// kept only if one of the user's roles (across every company they can
// reach) grants CanView on it, and a category is kept only if at least one
// of its children survived that filter. Unlike GetAppModules (which stays
// unfiltered for the Permission Settings admin UI, which must list every
// module regardless of the admin's own access), this is what the Sidebar,
// route table, and "Set it up" links should use.
func GetMyAppModules(c *gin.Context) {
	userIDVal, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "No role access configured"})
		return
	}
	userID, ok := userIDVal.(uint)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "No role access configured"})
		return
	}

	modules, err := authService().MyModules(userID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, modules)
}

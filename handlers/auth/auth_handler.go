package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/observability"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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
	UserID      uint   `json:"user_id"`
	CompaniesID uint   `json:"companies_id"`
	RolesID     uint   `json:"roles_id"`
	SessionID   string `json:"sid"`
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

// issueToken signs a token for an authenticated employee.
func issueToken(subject service.TokenSubject, sessionID string) (string, error) {
	secret, err := GetJWTSecret()
	if err != nil {
		return "", err
	}

	claims := TokenClaims{
		UserID:      subject.UserID,
		CompaniesID: subject.CompaniesID,
		RolesID:     subject.RolesID,
		SessionID:   sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(authTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func (h *Handler) Login(c *gin.Context) {
	var input models.LoginRequest
	if !httpx.BindJSON(c, &input) {
		return
	}
	input.Email = service.NormalizeLoginEmail(input.Email)
	if h.LoginAttempts != nil {
		if attempt := h.LoginAttempts.Check(input.Email); attempt.Limited {
			respondLoginRateLimited(c, attempt)
			return
		}
	}

	employee, err := h.Auth.Authenticate(input.Email, input.Password)
	if err != nil {
		if apperr.KindOf(err) == apperr.KindUnauthorized {
			slog.Warn("login failed",
				"request_id", observability.RequestID(c),
				"client_ip", c.ClientIP(),
				"reason", "invalid_credentials",
			)
			if h.LoginAttempts != nil {
				attempt := h.LoginAttempts.RecordFailure(input.Email)
				if attempt.Limited {
					respondLoginRateLimited(c, attempt)
					return
				}
				if attempt.Warn {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error": fmt.Sprintf(
							"Invalid email or password. You have made %d unsuccessful attempts. Further unsuccessful attempts will temporarily limit login.",
							attempt.Count,
						),
					})
					return
				}
			}
		}
		httpx.RespondError(c, err)
		return
	}
	if h.LoginAttempts != nil {
		h.LoginAttempts.Clear(input.Email)
	}

	secret, err := GetJWTSecret()
	if err != nil || len(secret) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication service is not configured"})
		return
	}

	sessionID, err := h.Sessions.Create(employee.ID, time.Now().Add(authTokenTTL))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create login session"})
		return
	}

	tokenString, err := issueToken(h.Auth.Subject(employee), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	SetAuthCookie(c, tokenString)
	c.JSON(http.StatusOK, h.Auth.Identity(employee))
}

func respondLoginRateLimited(c *gin.Context, attempt service.LoginAttemptResult) {
	seconds := int(attempt.RetryAfter.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error": "Too many unsuccessful login attempts. Please try again in 15 minutes.",
	})
}

func (h *Handler) Me(c *gin.Context) {
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

	identity, err := h.Auth.IdentityByID(id)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, identity)
}

func (h *Handler) Logout(c *gin.Context) {
	userID, userOK := c.Get("user_id")
	sessionID, sessionOK := c.Get("session_id")
	if userIDValue, ok := userID.(uint); userOK && ok {
		if sessionIDValue, ok := sessionID.(string); sessionOK && ok {
			if err := h.Sessions.Revoke(sessionIDValue, userIDValue, time.Now()); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not end login session"})
				return
			}
		}
	}
	ClearAuthCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

// GetAppModules returns every active module, unfiltered.
func (h *Handler) GetAppModules(c *gin.Context) {
	modules, err := h.Auth.AllModules()
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, modules)
}

// GetMyAppModules returns the active modules the caller can view, for the sidebar and route table.
func (h *Handler) GetMyAppModules(c *gin.Context) {
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

	modules, err := h.Auth.MyModules(userID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, modules)
}

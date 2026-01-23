package middleware

import (
	"net/http"
	"strings"

	"payroll-backend/config"
	"payroll-backend/handlers"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

/* --- AuthMiddleware validates JWT and extracts Tenant info (English Comment) --- */
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Get Token from Header (English Comment)
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// 2. Check "Bearer " prefix (English Comment)
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims := &handlers.TokenClaims{}

		// 3. Parse and Validate Token (English Comment)
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return handlers.GetJWTSecret(), nil // getter jwtSecret
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// 4. Set Tenant Data to Context (English Comment)
		c.Set("user_id", claims.UserID)
		c.Set("company_id", claims.CompanyID)
		c.Set("role_id", claims.RoleID)
		c.Set("is_super_admin", claims.IsSuperAdmin)

		c.Next()
	}
}

// CheckPermission validates if the current user has access to a specific module (English Comment)
func CheckPermission(moduleKey string, requireEdit bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Retrieve values from context (English Comment)
		valRole, roleExists := c.Get("role_id")
		valAdmin, adminExists := c.Get("is_super_admin")

		// 2. Strict check for context data (English Comment)
		if !roleExists || !adminExists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Security context missing"})
			c.Abort()
			return
		}

		// 3. Safe Type Conversion for RoleID (Support uint and float64) (English Comment)
		var rID uint
		switch v := valRole.(type) {
		case uint:
			rID = v
		case float64:
			rID = uint(v)
		case int:
			rID = uint(v)
		}

		// 4. Safe Type Assertion for IsSuperAdmin (English Comment)
		isSuper, _ := valAdmin.(bool)

		// 5. Super Admin Bypass: Grant all access (English Comment)
		if isSuper {
			c.Next()
			return
		}

		// 6. Database Authorization Check (English Comment)
		var access models.RoleModuleAccess
		err := config.DB.Table("role_module_accesses").
			Joins("JOIN app_modules ON app_modules.id = role_module_accesses.module_id").
			Where("role_module_accesses.role_id = ? AND app_modules.module_key = ?", rID, moduleKey).
			First(&access).Error

		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission for this module"})
			c.Abort()
			return
		}

		// 7. Action-level Validation (English Comment)
		if !access.CanView {
			c.JSON(http.StatusForbidden, gin.H{"error": "View access denied"})
			c.Abort()
			return
		}

		if requireEdit && !access.CanEdit {
			c.JSON(http.StatusForbidden, gin.H{"error": "Edit permission required"})
			c.Abort()
			return
		}

		// 8. Set DataScope for row-level filtering in Handlers (English Comment)
		c.Set("data_scope", access.DataScope)
		c.Next()
	}
}

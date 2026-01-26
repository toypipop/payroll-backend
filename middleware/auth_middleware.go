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

// CheckPermission validates specific action access for a module (English Comment)
func CheckPermission(moduleKey string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Retrieve values from context (English Comment)
		valRole, roleExists := c.Get("role_id")
		valAdmin, adminExists := c.Get("is_super_admin")

		if !roleExists || !adminExists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Security context missing"})
			c.Abort()
			return
		}

		// 2. Safe Type Conversion for RoleID (English Comment)
		var rID uint
		switch v := valRole.(type) {
		case uint:
			rID = v
		case float64:
			rID = uint(v)
		case int:
			rID = uint(v)
		}

		isSuper, _ := valAdmin.(bool)

		// 3. Super Admin Bypass (English Comment)
		if isSuper {
			c.Set("data_scope", "ALL") // Super Admin always sees everything
			c.Next()
			return
		}

		// 4. Database Authorization Check (English Comment)
		var access models.RoleModuleAccess
		err := config.DB.Table("role_module_accesses").
			Joins("JOIN app_modules ON app_modules.id = role_module_accesses.module_id").
			Where("role_module_accesses.role_id = ? AND app_modules.module_key = ?", rID, moduleKey).
			First(&access).Error

		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied for this module"})
			c.Abort()
			return
		}

		// 5. Action-level Validation based on new fields (English Comment)
		allowed := false
		switch action {
		case "VIEW":
			allowed = access.CanView
		case "CREATE":
			allowed = access.CanCreate
		case "EDIT":
			allowed = access.CanEdit
		case "DELETE":
			allowed = access.CanDelete
		default:
			allowed = false
		}

		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied for action: " + action})
			c.Abort()
			return
		}

		// 6. Set DataScope for row-level filtering in Handlers (English Comment)
		c.Set("data_scope", access.DataScope)
		c.Next()
	}
}

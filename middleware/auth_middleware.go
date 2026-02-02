package middleware

import (
	"log"
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

func CheckPermission(moduleKey string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		valRole, _ := c.Get("role_id")

		var rID uint
		switch v := valRole.(type) {
		case uint:
			rID = v
		case float64:
			rID = uint(v)
		case int:
			rID = uint(v)
		}

		var access models.RoleModuleAccess

		// DEBUG 1: Use Debug() to see the generated SQL in console (English Comment)
		query := config.DB.Debug().InnerJoins("AppModule").
			Where(&models.RoleModuleAccess{RoleID: rID}).
			Where("`AppModule`.`module_key` = ?", moduleKey) // Use backticks for safety

		err := query.First(&access).Error

		if err != nil {
			// DEBUG 2: Log details to find if it's 'Record Not Found' or 'SQL Error' (English Comment)
			log.Printf("[DEBUG] ModuleKey: %s, RoleID: %d, Error: %v", moduleKey, rID, err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied", "debug_msg": err.Error()})
			c.Abort()
			return
		}

		// DEBUG 3: Log the loaded permissions (English Comment)
		log.Printf("[DEBUG] Permissions Loaded: %+v", access)

		permissions := map[string]bool{
			"VIEW":   access.CanView,
			"CREATE": access.CanCreate,
			"EDIT":   access.CanEdit,
			"DELETE": access.CanDelete,
		}

		allowed, ok := permissions[action]
		if !ok || !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error":  "Permission denied",
				"action": action,
				"status": allowed,
			})
			c.Abort()
			return
		}

		c.Set("data_scope", access.DataScope)
		c.Next()
	}
}

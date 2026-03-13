package middleware

import (
	"log"
	"net/http"
	"strings"

	"payroll-backend/config"
	authhandler "payroll-backend/handlers/auth"
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
		claims := &authhandler.TokenClaims{}

		// 3. Parse and Validate Token (English Comment)
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return authhandler.GetJWTSecret(), nil // getter jwtSecret
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// 4. Set Tenant Data to Context (English Comment)
		c.Set("user_id", claims.UserID)
		c.Set("companies_id", claims.CompaniesID)
		c.Set("roles_id", claims.RolesID)
		c.Set("is_super_admin", claims.IsSuperAdmin)

		c.Next()
	}
}

func CheckPermission(moduleKey string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		valRoles, _ := c.Get("roles_id")

		var rID uint
		switch v := valRoles.(type) {
		case uint:
			rID = v
		case float64:
			rID = uint(v)
		case int:
			rID = uint(v)
		}

		var access models.RolesModuleAccess

		// Use struct for Where to avoid hardcoded table names (English Comment)
		query := config.DB.Debug().
			InnerJoins("AppModule", config.DB.Where("`AppModule`.`deleted_at` = ?", 0)).
			Where(&models.RolesModuleAccess{RolesID: rID}).
			Where("`AppModule`.`module_key` = ?", moduleKey)

		err := query.First(&access).Error

		if err != nil {
			log.Printf("[DEBUG] ModuleKey: %s, RolesID: %d, Error: %v", moduleKey, rID, err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied", "debug_msg": err.Error()})
			c.Abort()
			return
		}

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

package middleware

import (
	"log"
	"net/http"

	"payroll-backend/config"
	authhandler "payroll-backend/handlers/auth"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware validates JWT and extracts tenant
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie(authhandler.GetAuthCookieName())
		if err != nil || tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication cookie is required"})
			c.Abort()
			return
		}

		claims := &authhandler.TokenClaims{}

		jwtSecret, err := authhandler.GetJWTSecret()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication service is not configured"})
			c.Abort()
			return
		}

		// Parse and validate JWT token
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set tenant data to context
		c.Set("user_id", claims.UserID)
		companyIDs := repository.AccessibleCompanyIDs(claims.UserID)
		roleIDs := repository.AccessibleRoleIDs(claims.UserID)
		if len(companyIDs) == 0 || len(roleIDs) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "No company access configured"})
			c.Abort()
			return
		}
		c.Set("companies_id", companyIDs[0])
		c.Set("accessible_company_ids", companyIDs)
		c.Set("role_ids", roleIDs)
		if len(roleIDs) > 0 {
			c.Set("roles_id", roleIDs[0])
		}

		c.Next()
	}
}

func scopeRank(scope models.DataScope) int {
	switch scope {
	case models.ScopeAll:
		return 4
	case models.ScopeCompanies:
		return 3
	case models.ScopeDepartment:
		return 2
	case models.ScopeOwn:
		return 1
	default:
		return 0
	}
}

func getContextRoleIDs(c *gin.Context) []uint {
	if val, ok := c.Get("role_ids"); ok {
		if ids, ok := val.([]uint); ok {
			return ids
		}
	}

	valRoles, _ := c.Get("roles_id")
	switch v := valRoles.(type) {
	case uint:
		return []uint{v}
	case float64:
		return []uint{uint(v)}
	case int:
		return []uint{uint(v)}
	default:
		return nil
	}
}

func CheckPermission(moduleKey string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleIDs := getContextRoleIDs(c)
		if len(roleIDs) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "No role access configured"})
			c.Abort()
			return
		}

		var accesses []models.RolesModuleAccess
		err := config.DB.
			InnerJoins("AppModule", config.DB.Where("`AppModule`.`deleted_at` = ?", 0)).
			Where("roles_module_access.roles_id IN ?", roleIDs).
			Where("`AppModule`.`module_key` = ?", moduleKey).
			Find(&accesses).Error

		if err != nil || len(accesses) == 0 {
			log.Printf("[DEBUG] ModuleKey: %s, RoleIDs: %v, Error: %v", moduleKey, roleIDs, err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			c.Abort()
			return
		}

		var selected *models.RolesModuleAccess
		var selectedRoleIDs []uint
		for i := range accesses {
			access := &accesses[i]
			permissions := map[string]bool{
				"VIEW":   access.CanView,
				"CREATE": access.CanCreate,
				"EDIT":   access.CanEdit,
				"DELETE": access.CanDelete,
			}
			if allowed, ok := permissions[action]; ok && allowed {
				if selected == nil || scopeRank(access.DataScope) > scopeRank(selected.DataScope) {
					selected = access
					selectedRoleIDs = []uint{access.RolesID}
				} else if selected != nil && scopeRank(access.DataScope) == scopeRank(selected.DataScope) {
					selectedRoleIDs = append(selectedRoleIDs, access.RolesID)
				}
			}
		}

		if selected == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error":  "Permission denied",
				"action": action,
			})
			c.Abort()
			return
		}

		c.Set("data_scope", selected.DataScope)
		userID, _ := c.Get("user_id")
		if id, ok := userID.(uint); ok {
			c.Set("authorized_company_ids", repository.AccessibleCompanyIDsForRoles(id, selectedRoleIDs))
		}
		c.Next()
	}
}

func CheckPermissionAny(moduleKeys []string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(moduleKeys) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			c.Abort()
			return
		}

		roleIDs := getContextRoleIDs(c)
		if len(roleIDs) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "No role access configured"})
			c.Abort()
			return
		}

		var accesses []models.RolesModuleAccess
		err := config.DB.
			InnerJoins("AppModule", config.DB.Where("`AppModule`.`deleted_at` = ?", 0)).
			Where("roles_module_access.roles_id IN ?", roleIDs).
			Where("`AppModule`.`module_key` IN ?", moduleKeys).
			Find(&accesses).Error

		if err != nil || len(accesses) == 0 {
			log.Printf("[DEBUG] ModuleKeys: %v, RoleIDs: %v, Error: %v", moduleKeys, roleIDs, err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			c.Abort()
			return
		}

		var selected *models.RolesModuleAccess
		var selectedRoleIDs []uint
		for _, access := range accesses {
			permissions := map[string]bool{
				"VIEW":   access.CanView,
				"CREATE": access.CanCreate,
				"EDIT":   access.CanEdit,
				"DELETE": access.CanDelete,
			}

			allowed, ok := permissions[action]
			if ok && allowed {
				if selected == nil || scopeRank(access.DataScope) > scopeRank(selected.DataScope) {
					selected = &access
					selectedRoleIDs = []uint{access.RolesID}
				} else if selected != nil && scopeRank(access.DataScope) == scopeRank(selected.DataScope) {
					selectedRoleIDs = append(selectedRoleIDs, access.RolesID)
				}
			}
		}

		if selected != nil {
			c.Set("data_scope", selected.DataScope)
			userID, _ := c.Get("user_id")
			if id, ok := userID.(uint); ok {
				c.Set("authorized_company_ids", repository.AccessibleCompanyIDsForRoles(id, selectedRoleIDs))
			}
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":  "Permission denied",
			"action": action,
		})
		c.Abort()
	}
}

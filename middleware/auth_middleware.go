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

const (
	appModuleDeletedAtColumn = `"AppModule"."deleted_at"`
	appModuleIsActiveColumn  = `"AppModule"."is_active"`
	appModuleModuleKeyColumn = `"AppModule"."module_key"`
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

func getContextUserID(c *gin.Context) (uint, bool) {
	val, _ := c.Get("user_id")
	id, ok := val.(uint)
	return id, ok
}

func permissionAllows(access *models.RolesModuleAccess, action string) bool {
	switch action {
	case "VIEW":
		return access.CanView
	case "CREATE":
		return access.CanCreate
	case "EDIT":
		return access.CanEdit
	case "DELETE":
		return access.CanDelete
	default:
		return false
	}
}

// resolveAuthorizedCompanies keeps only the companies whose role actually grants the action.
func resolveAuthorizedCompanies(companyRoles []repository.CompanyRole, accesses []models.RolesModuleAccess, action string) ([]uint, models.DataScope, bool) {
	type roleResult struct {
		allowed bool
		scope   models.DataScope
	}
	roleResults := make(map[uint]roleResult)
	for i := range accesses {
		access := &accesses[i]
		if !permissionAllows(access, action) {
			continue
		}
		cur := roleResults[access.RolesID]
		if !cur.allowed || scopeRank(access.DataScope) > scopeRank(cur.scope) {
			roleResults[access.RolesID] = roleResult{allowed: true, scope: access.DataScope}
		}
	}

	var companyIDs []uint
	var bestScope models.DataScope
	found := false
	for _, cr := range companyRoles {
		res, ok := roleResults[cr.RoleID]
		if !ok || !res.allowed {
			continue
		}
		companyIDs = append(companyIDs, cr.CompanyID)
		if !found || scopeRank(res.scope) > scopeRank(bestScope) {
			bestScope = res.scope
			found = true
		}
	}
	return companyIDs, bestScope, found
}

func CheckPermission(moduleKey string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getContextUserID(c)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "No role access configured"})
			c.Abort()
			return
		}

		companyRoles := repository.AccessibleCompanyRoles(userID)
		if len(companyRoles) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "No role access configured"})
			c.Abort()
			return
		}

		roleIDs := make([]uint, 0, len(companyRoles))
		for _, cr := range companyRoles {
			roleIDs = append(roleIDs, cr.RoleID)
		}

		var accesses []models.RolesModuleAccess
		// PostgreSQL does not support MySQL backtick quoting; the mixed-case GORM association alias must use double-quoted identifiers here.
		err := config.DB.
			InnerJoins("AppModule", config.DB.Where(appModuleDeletedAtColumn+" = ?", 0)).
			Where("roles_module_access.roles_id IN ?", roleIDs).
			Where(appModuleModuleKeyColumn+" = ?", moduleKey).
			Where(appModuleIsActiveColumn+" = ?", true).
			Find(&accesses).Error

		if err != nil || len(accesses) == 0 {
			log.Printf("[DEBUG] ModuleKey: %s, RoleIDs: %v, Error: %v", moduleKey, roleIDs, err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			c.Abort()
			return
		}

		companyIDs, scope, found := resolveAuthorizedCompanies(companyRoles, accesses, action)
		if !found {
			c.JSON(http.StatusForbidden, gin.H{
				"error":  "Permission denied",
				"action": action,
			})
			c.Abort()
			return
		}

		c.Set("data_scope", scope)
		c.Set("authorized_company_ids", companyIDs)
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

		userID, ok := getContextUserID(c)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "No role access configured"})
			c.Abort()
			return
		}

		companyRoles := repository.AccessibleCompanyRoles(userID)
		if len(companyRoles) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "No role access configured"})
			c.Abort()
			return
		}

		roleIDs := make([]uint, 0, len(companyRoles))
		for _, cr := range companyRoles {
			roleIDs = append(roleIDs, cr.RoleID)
		}

		var accesses []models.RolesModuleAccess
		// PostgreSQL does not support MySQL backtick quoting; the mixed-case GORM association alias must use double-quoted identifiers here.
		err := config.DB.
			InnerJoins("AppModule", config.DB.Where(appModuleDeletedAtColumn+" = ?", 0)).
			Where("roles_module_access.roles_id IN ?", roleIDs).
			Where(appModuleModuleKeyColumn+" IN ?", moduleKeys).
			Where(appModuleIsActiveColumn+" = ?", true).
			Find(&accesses).Error

		if err != nil || len(accesses) == 0 {
			log.Printf("[DEBUG] ModuleKeys: %v, RoleIDs: %v, Error: %v", moduleKeys, roleIDs, err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			c.Abort()
			return
		}

		companyIDs, scope, found := resolveAuthorizedCompanies(companyRoles, accesses, action)
		if !found {
			c.JSON(http.StatusForbidden, gin.H{
				"error":  "Permission denied",
				"action": action,
			})
			c.Abort()
			return
		}

		c.Set("data_scope", scope)
		c.Set("authorized_company_ids", companyIDs)
		c.Next()
	}
}

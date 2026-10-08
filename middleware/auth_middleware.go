package middleware

import (
	"log/slog"
	"net/http"
	"time"

	authhandler "payroll-backend/handlers/auth"
	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/observability"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Guard holds the lookups the request-gating middleware needs, so no middleware reaches for a database handle of its own.
type Guard struct {
	access    repository.AccessRepository
	delete    repository.DeleteGuardRepository
	companies *service.CompanyAccessService
	sessions  *service.SessionService
}

// NewGuard builds the middleware set from the repositories main composed.
func NewGuard(access repository.AccessRepository, deleteGuard repository.DeleteGuardRepository, companies *service.CompanyAccessService, sessions *service.SessionService) *Guard {
	return &Guard{access: access, delete: deleteGuard, companies: companies, sessions: sessions}
}

// Auth validates the JWT cookie and puts the caller's identity and reachable companies on the request.
func (g *Guard) Auth() gin.HandlerFunc {
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
			authhandler.ClearAuthCookie(c)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}
		if claims.SessionID == "" {
			authhandler.ClearAuthCookie(c)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}
		active, err := g.sessions.IsActive(claims.SessionID, claims.UserID, time.Now())
		if err != nil {
			slog.Error("authentication session lookup failed", "request_id", observability.RequestID(c))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication service is unavailable"})
			c.Abort()
			return
		}
		if !active {
			authhandler.ClearAuthCookie(c)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Re-read account status on every request, including existing cookie sessions.
		user, err := g.access.FindScopeUser(claims.UserID)
		if err != nil {
			authhandler.ClearAuthCookie(c)
			httpx.RespondError(c, service.ErrAuthUserNotLoaded)
			c.Abort()
			return
		}
		if !user.IsActive {
			authhandler.ClearAuthCookie(c)
			httpx.RespondError(c, service.ErrAuthInactive)
			c.Abort()
			return
		}
		c.Set("scope_user", user)

		// Set tenant data to context
		c.Set("user_id", claims.UserID)
		c.Set("session_id", claims.SessionID)
		access, err := g.access.ResolveAccess(user)
		if err != nil {
			slog.Error("company access lookup failed", "request_id", observability.RequestID(c))
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			c.Abort()
			return
		}
		companyIDs, roleIDs := access.CompanyIDs, access.RoleIDs
		c.Set("company_roles", access.CompanyRoles)
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

// CheckPermission gates the request on the module granting the action, and publishes the scope it resolved.
func (g *Guard) CheckPermission(moduleKey string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, ok := getContextUserID(c)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "No role access configured"})
			c.Abort()
			return
		}

		value, _ := c.Get("company_roles")
		companyRoles, _ := value.([]repository.CompanyRole)
		if value, limited := c.Get("active_company_ids"); limited {
			active, _ := value.([]uint)
			companyRoles = activeCompanyRoles(companyRoles, active)
		}
		if len(companyRoles) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "No role access configured"})
			c.Abort()
			return
		}

		roleIDs := make([]uint, 0, len(companyRoles))
		seenRoles := make(map[uint]bool, len(companyRoles))
		for _, cr := range companyRoles {
			if !seenRoles[cr.RoleID] {
				seenRoles[cr.RoleID] = true
				roleIDs = append(roleIDs, cr.RoleID)
			}
		}

		accesses, err := g.access.ModuleAccesses(roleIDs, moduleKey)
		if err != nil {
			slog.Error("permission lookup failed",
				"request_id", observability.RequestID(c),
				"module", moduleKey,
				"action", action,
				"error", err,
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			c.Abort()
			return
		}
		if len(accesses) == 0 {
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

func activeCompanyRoles(roles []repository.CompanyRole, active []uint) []repository.CompanyRole {
	allowed := make(map[uint]bool, len(active))
	for _, id := range active {
		allowed[id] = true
	}
	result := make([]repository.CompanyRole, 0, len(roles))
	for _, role := range roles {
		if allowed[role.CompanyID] {
			result = append(result, role)
		}
	}
	return result
}

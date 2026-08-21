package httpx

import (
	"net/http"
	"strconv"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// RequestScope resolves the caller's authorization context from what AuthMiddleware and CheckPermission stored, or answers 401 and reports false.
func RequestScope(c *gin.Context) (repository.AccessScope, bool) {
	val, ok := c.Get("scope_user")
	user, isUser := val.(models.Employees)
	if !ok || !isUser || user.ID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return repository.AccessScope{}, false
	}
	_, dataScope := ContextAuth(c)
	return repository.NewAccessScope(user, dataScope, AuthorizedCompanyIDs(c)), true
}

// ContextAuth reads the authenticated user id and the data scope CheckPermission resolved for this route.
func ContextAuth(c *gin.Context) (uint, models.DataScope) {
	valUser, _ := c.Get("user_id")
	var userID uint
	switch v := valUser.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	}

	valScope, _ := c.Get("data_scope")
	scope, _ := valScope.(models.DataScope)

	return userID, scope
}

// AuthorizedCompanyIDs prefers the companies whose role grants this route's action, falling back to every company the caller can reach on routes with no permission gate.
func AuthorizedCompanyIDs(c *gin.Context) []uint {
	if val, ok := c.Get("authorized_company_ids"); ok {
		if ids, ok := val.([]uint); ok {
			return ids
		}
	}
	if val, ok := c.Get("accessible_company_ids"); ok {
		if ids, ok := val.([]uint); ok {
			return ids
		}
	}
	return nil
}

// ParseID reads a positive uint route param, or answers 400 and reports false.
func ParseID(c *gin.Context, param string, label string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid " + label + " id"})
		return 0, false
	}
	return uint(id), true
}

// BindJSON decodes the request body into dst, or answers 400 and reports false.
func BindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return false
	}
	return true
}

// RespondError maps a domain error onto a status code without leaking driver or SQL detail.
func RespondError(c *gin.Context, err error) {
	c.JSON(statusFor(apperr.KindOf(err)), gin.H{"error": apperr.Message(err)})
}

// statusFor is the single place the whole API decides which status an error kind deserves.
func statusFor(kind apperr.Kind) int {
	switch kind {
	case apperr.KindValidation:
		return http.StatusBadRequest
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

package httpx

import (
	"net/http"
	"strconv"

	"payroll-backend/apperr"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// RequestScope resolves the caller's authorization context, or answers 401 and reports false.
func RequestScope(c *gin.Context) (repository.AccessScope, bool) {
	scope, err := repository.NewAccessScope(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return repository.AccessScope{}, false
	}
	return scope, true
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

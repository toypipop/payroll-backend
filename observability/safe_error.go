package observability

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

var sqlStatePattern = regexp.MustCompile(`^[A-Z0-9]{5}$`)

// ErrorAttrs deliberately excludes error messages, SQL, driver details and values.
func ErrorAttrs(err error) []any {
	attrs := []any{"error_type", fmt.Sprintf("%T", err)}
	var state interface{ SQLState() string }
	if errors.As(err, &state) && sqlStatePattern.MatchString(state.SQLState()) {
		attrs = append(attrs, "sql_state", state.SQLState())
	}
	return attrs
}

// Route never returns raw paths or query strings, including unmatched routes.
func Route(c *gin.Context) string {
	if route := c.FullPath(); route != "" {
		return route
	}
	return "<unmatched>"
}

func Method(c *gin.Context) string {
	switch c.Request.Method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return c.Request.Method
	default:
		return "OTHER"
	}
}

func LogError(message string, err error) {
	slog.Error(message, ErrorAttrs(err)...)
}

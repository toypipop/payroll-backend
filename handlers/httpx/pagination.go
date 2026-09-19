package httpx

import (
	"github.com/gin-gonic/gin"
	"payroll-backend/apperr"
	"payroll-backend/repository"
	"strconv"
	"strings"
)

func ParsePage(c *gin.Context) (repository.PageQuery, bool) {
	page, errPage := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, errSize := strconv.Atoi(c.DefaultQuery("page_size", "25"))
	q := repository.PageQuery{Page: page, PageSize: size, Search: strings.TrimSpace(c.Query("search")), Sort: c.Query("sort"), Order: c.DefaultQuery("order", "asc")}
	if errPage != nil || errSize != nil {
		RespondError(c, apperr.Validation("page and page_size must be integers"))
		return q, false
	}
	if err := q.Validate(); err != nil {
		RespondError(c, err)
		return q, false
	}
	return q, true
}

package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetDepartmentCategories(c *gin.Context) {
	categories, err := h.DepartmentCategory.List()
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, categories)
}

func (h *Handler) CreateDepartmentCategory(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	category, err := h.DepartmentCategory.Create(scope, input.Name)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, category)
}

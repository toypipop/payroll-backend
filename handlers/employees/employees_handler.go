package employees

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetEmployees(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	employees, err := h.Employees.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, employees)
}

func (h *Handler) GetEmployeeOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	options, err := h.Employees.Options(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, options)
}

func (h *Handler) CreateEmployees(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	input := models.Employees{IsActive: true}
	if !httpx.BindJSON(c, &input) {
		return
	}

	created, err := h.Employees.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *Handler) UpdateEmployees(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee")
	if !ok {
		return
	}

	var patch map[string]any
	if !httpx.BindJSON(c, &patch) {
		return
	}

	updated, err := h.Employees.Update(scope, id, patch)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *Handler) DeleteEmployees(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee")
	if !ok {
		return
	}

	if err := h.Employees.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

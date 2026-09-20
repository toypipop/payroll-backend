package employees

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetEmployeeEarnings(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	items, err := h.EmployeeEarning.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateEmployeeEarning(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.EmployeeEarning
	if !httpx.BindJSON(c, &input) {
		return
	}

	earning, err := h.EmployeeEarning.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, earning)
}

func (h *Handler) UpdateEmployeeEarning(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee earning")
	if !ok {
		return
	}

	var input models.EmployeeEarning
	if !httpx.BindJSON(c, &input) {
		return
	}

	earning, err := h.EmployeeEarning.Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, earning)
}

func (h *Handler) DeleteEmployeeEarning(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee earning")
	if !ok {
		return
	}

	if err := h.EmployeeEarning.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

package employees

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetEmployeeDeductions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	items, err := h.EmployeeDeduction.List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateEmployeeDeduction(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.EmployeeDeduction
	if !httpx.BindJSON(c, &input) {
		return
	}

	deduction, err := h.EmployeeDeduction.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, deduction)
}

func (h *Handler) UpdateEmployeeDeduction(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee deduction")
	if !ok {
		return
	}

	var input models.EmployeeDeduction
	if !httpx.BindJSON(c, &input) {
		return
	}

	deduction, err := h.EmployeeDeduction.Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, deduction)
}

func (h *Handler) DeleteEmployeeDeduction(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "employee deduction")
	if !ok {
		return
	}

	if err := h.EmployeeDeduction.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

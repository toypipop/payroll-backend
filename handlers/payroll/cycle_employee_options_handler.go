package payroll

import (
	"net/http"

	"payroll-backend/handlers/httpx"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetCycleEmployeeOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "payroll cycle")
	if !ok {
		return
	}
	options, err := h.PayrollRun.CycleEmployeeOptions(scope, id)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, options)
}

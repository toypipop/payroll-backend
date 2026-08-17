package payroll

import (
	"net/http"
	"strconv"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

// queryCycleID reads the cycle_id query parameter, returning 0 when it is absent or unparseable so the service can answer with its own message.
func queryCycleID(c *gin.Context) uint {
	value, err := strconv.ParseUint(c.Query("cycle_id"), 10, 64)
	if err != nil {
		return 0
	}
	return uint(value)
}

func (h *Handler) GetCycleEarnings(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	items, err := h.CycleEarning.List(scope, queryCycleID(c))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateCycleEarning(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.PayrollCycleEarning
	if !httpx.BindJSON(c, &input) {
		return
	}

	item, err := h.CycleEarning.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateCycleEarning(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "cycle earning")
	if !ok {
		return
	}

	var input models.PayrollCycleEarning
	if !httpx.BindJSON(c, &input) {
		return
	}

	item, err := h.CycleEarning.Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteCycleEarning(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "cycle earning")
	if !ok {
		return
	}

	if err := h.CycleEarning.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

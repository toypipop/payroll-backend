package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetHolidays(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	holidays, err := h.Holiday.List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, holidays)
}

func (h *Handler) CreateHoliday(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input struct {
		CompaniesID uint   `json:"companies_id"`
		Date        string `json:"date"`
		Name        string `json:"name"`
		IsPaid      bool   `json:"is_paid"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	holiday, err := h.Holiday.Create(scope, service.HolidayInput{
		CompaniesID: input.CompaniesID,
		Date:        input.Date,
		Name:        input.Name,
		IsPaid:      input.IsPaid,
	})
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, holiday)
}

func (h *Handler) UpdateHoliday(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "holiday")
	if !ok {
		return
	}

	// companies_id is not accepted here: the company is immutable after creation.
	var input struct {
		Date   string `json:"date"`
		Name   string `json:"name"`
		IsPaid bool   `json:"is_paid"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	holiday, err := h.Holiday.Update(scope, id, service.HolidayInput{
		Date:   input.Date,
		Name:   input.Name,
		IsPaid: input.IsPaid,
	})
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, holiday)
}

func (h *Handler) DeleteHoliday(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "holiday")
	if !ok {
		return
	}

	if err := h.Holiday.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Holiday deleted"})
}

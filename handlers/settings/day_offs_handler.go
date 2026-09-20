package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// dayOffRequest is the wire shape for both create and update.
type dayOffRequest struct {
	CompaniesID uint   `json:"companies_id"`
	Weekday     int    `json:"weekday"`
	Name        string `json:"name"`
	IsPaid      bool   `json:"is_paid"`
}

func (h *Handler) GetDayOffs(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	dayOffs, err := h.DayOff.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, dayOffs)
}

// GetDayOffOptions returns a minimal id/weekday/companies_id list.
func (h *Handler) GetDayOffOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	dayOffs, err := h.DayOff.Options(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type option struct {
		ID          uint `json:"id"`
		Weekday     int  `json:"weekday"`
		CompaniesID uint `json:"companies_id"`
	}
	options := make([]option, 0, len(dayOffs))
	for _, d := range dayOffs {
		options = append(options, option{ID: d.ID, Weekday: d.Weekday, CompaniesID: d.CompaniesID})
	}
	c.JSON(http.StatusOK, options)
}

func (h *Handler) GetDayOffCompanies(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	companies, err := h.DayOff.Companies(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, companies)
}

func (h *Handler) CreateDayOff(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input dayOffRequest
	if !httpx.BindJSON(c, &input) {
		return
	}

	dayOff, err := h.DayOff.Create(scope, service.DayOffInput{
		CompaniesID: input.CompaniesID,
		Weekday:     input.Weekday,
		Name:        input.Name,
		IsPaid:      input.IsPaid,
	})
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dayOff)
}

func (h *Handler) UpdateDayOff(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "day off")
	if !ok {
		return
	}

	// companies_id is accepted and ignored: the company is immutable after creation.
	var input dayOffRequest
	if !httpx.BindJSON(c, &input) {
		return
	}

	dayOff, err := h.DayOff.Update(scope, id, service.DayOffInput{
		Weekday: input.Weekday,
		Name:    input.Name,
		IsPaid:  input.IsPaid,
	})
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, dayOff)
}

func (h *Handler) DeleteDayOff(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "day off")
	if !ok {
		return
	}

	if err := h.DayOff.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Day off deleted"})
}

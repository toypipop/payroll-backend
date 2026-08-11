package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// holidayService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func holidayService() *service.HolidayService {
	return service.NewHolidayService(repository.NewHolidayRepository())
}

func GetHolidays(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	holidays, err := holidayService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, holidays)
}

func CreateHoliday(c *gin.Context) {
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

	holiday, err := holidayService().Create(scope, service.HolidayInput{
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

func UpdateHoliday(c *gin.Context) {
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

	holiday, err := holidayService().Update(scope, id, service.HolidayInput{
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

func DeleteHoliday(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "holiday")
	if !ok {
		return
	}

	if err := holidayService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Holiday deleted"})
}

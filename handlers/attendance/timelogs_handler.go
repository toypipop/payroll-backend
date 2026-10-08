package attendance

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetTimeLogs(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	logs, err := h.TimeLog.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (h *Handler) CreateTimeLog(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input service.TimeLogInput
	if !httpx.BindJSON(c, &input) {
		return
	}

	log, err := h.TimeLog.Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, log)
}

// BulkCreateTimeLog creates one TimeLog per matching weekday within a date
func (h *Handler) BulkCreateTimeLog(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input struct {
		EmployeesID    uint   `json:"employees_id" binding:"required"`
		CompaniesID    uint   `json:"companies_id" binding:"required"`
		WorkLocationID uint   `json:"work_location_id" binding:"required"`
		StartDate      string `json:"start_date" binding:"required"`
		EndDate        string `json:"end_date" binding:"required"`
		Weekdays       []int  `json:"weekdays" binding:"required"`
		CheckInTime    string `json:"check_in_time" binding:"required"`
		CheckOutTime   string `json:"check_out_time" binding:"required"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	result, err := h.TimeLog.BulkCreate(scope, service.BulkTimeLogInput{
		EmployeesID:    input.EmployeesID,
		CompaniesID:    input.CompaniesID,
		WorkLocationID: input.WorkLocationID,
		StartDate:      input.StartDate,
		EndDate:        input.EndDate,
		Weekdays:       input.Weekdays,
		CheckInTime:    input.CheckInTime,
		CheckOutTime:   input.CheckOutTime,
	})
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"created_count": result.CreatedCount,
		"skipped_count": len(result.SkippedDates),
		"skipped_dates": result.SkippedDates,
		"holiday_count": len(result.HolidayDates),
		"holiday_dates": result.HolidayDates,
	})
}

func (h *Handler) UpdateTimeLog(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "time log")
	if !ok {
		return
	}

	var input service.TimeLogInput
	if !httpx.BindJSON(c, &input) {
		return
	}

	log, err := h.TimeLog.Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, log)
}

func (h *Handler) DeleteTimeLog(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "time log")
	if !ok {
		return
	}

	if err := h.TimeLog.Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

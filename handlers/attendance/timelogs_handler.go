package attendance

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// timeLogService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func timeLogService() *service.TimeLogService {
	return service.NewTimeLogService(repository.NewTimeLogRepository(), repository.NewEmployeesRepository())
}

func GetTimeLogs(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	logs, err := timeLogService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, logs)
}

func CreateTimeLog(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.TimeLog
	if !httpx.BindJSON(c, &input) {
		return
	}

	log, err := timeLogService().Create(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, log)
}

// BulkCreateTimeLog creates one TimeLog per matching weekday within a date
// range for a single employee, using the same check-in/check-out time of
// day for every generated log. Dates that already have a log for that
// employee are skipped rather than duplicated.
func BulkCreateTimeLog(c *gin.Context) {
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

	result, err := timeLogService().BulkCreate(scope, service.BulkTimeLogInput{
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
	})
}

func UpdateTimeLog(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "time log")
	if !ok {
		return
	}

	var input models.TimeLog
	if !httpx.BindJSON(c, &input) {
		return
	}

	log, err := timeLogService().Update(scope, id, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, log)
}

func DeleteTimeLog(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "time log")
	if !ok {
		return
	}

	if err := timeLogService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully"})
}

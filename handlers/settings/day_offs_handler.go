package settings

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// dayOffService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func dayOffService() *service.DayOffService {
	return service.NewDayOffService(repository.NewDayOffRepository(), repository.NewCompanyRepository())
}

// dayOffRequest is the wire shape for both create and update.
type dayOffRequest struct {
	CompaniesID uint   `json:"companies_id"`
	Weekday     int    `json:"weekday"`
	Name        string `json:"name"`
	IsPaid      bool   `json:"is_paid"`
}

func GetDayOffs(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	dayOffs, err := dayOffService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, dayOffs)
}

// GetDayOffOptions returns a minimal id/weekday/companies_id list. Ungated by
// page permission (mirrors GetWorkLocationOptions) so other pages (e.g. the
// Time Log bulk-add form) can default weekday selection to a company's
// configured day-offs without needing DayOffPage permission.
func GetDayOffOptions(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	dayOffs, err := dayOffService().Options(scope)
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

func GetDayOffCompanies(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	companies, err := dayOffService().Companies(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, companies)
}

func CreateDayOff(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input dayOffRequest
	if !httpx.BindJSON(c, &input) {
		return
	}

	dayOff, err := dayOffService().Create(scope, service.DayOffInput{
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

func UpdateDayOff(c *gin.Context) {
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

	dayOff, err := dayOffService().Update(scope, id, service.DayOffInput{
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

func DeleteDayOff(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "day off")
	if !ok {
		return
	}

	if err := dayOffService().Delete(scope, id); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Day off deleted"})
}

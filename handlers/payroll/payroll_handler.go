package payroll

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// payrollConfigService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func payrollConfigService() *service.PayrollConfigService {
	return service.NewPayrollConfigService(repository.NewPayrollConfigRepository())
}

// GetPayrollConfigs starts from companies so a company with no configuration yet
// still appears in the list.
func GetPayrollConfigs(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	companies, err := payrollConfigService().List(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, companies)
}

// UpdatePayrollConfig saves a company's configuration, updating the existing row
// when there is one: companies_id is unique, so a company has exactly one.
func UpdatePayrollConfig(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input models.PayrollConfig
	if !httpx.BindJSON(c, &input) {
		return
	}

	saved, err := payrollConfigService().Save(scope, input)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Configuration saved", "data": saved})
}

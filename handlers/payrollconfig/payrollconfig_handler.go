package payrollconfig

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// payrollConfigService is resolved lazily so the package never captures config.DB before ConnectDatabase runs.
func payrollConfigService() *service.PayrollConfigService {
	return service.NewPayrollConfigService(repository.NewPayrollConfigRepository())
}

// GetConfigByPayrollCycle returns the configuration in force for a cycle's company.
func GetConfigByPayrollCycle(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "cycle_id", "payroll cycle")
	if !ok {
		return
	}

	result, err := payrollConfigService().ForCycle(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config":       result.Config,
		"cycle_id":     result.CycleID,
		"companies_id": result.CompaniesID,
	})
}

package payroll

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// GetMySlips returns the caller's own payslips.
func GetMySlips(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	slips, err := payrollRunService().MySlips(userID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type slipRow struct {
		models.PayrollTransaction
		CycleName string `json:"cycle_name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	result := make([]slipRow, 0, len(slips))
	for _, slip := range slips {
		result = append(result, slipRow{
			PayrollTransaction: slip.Transaction,
			CycleName:          slip.CycleName,
			StartDate:          slip.StartDate,
			EndDate:            slip.EndDate,
		})
	}
	c.JSON(http.StatusOK, result)
}

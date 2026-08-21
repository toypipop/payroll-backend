package payroll

import (
	"net/http"

	"payroll-backend/apperr"
	"payroll-backend/handlers/httpx"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
)

// GetMySlips returns the caller's own payslips.
func (h *Handler) GetMySlips(c *gin.Context) {
	userID, _ := httpx.ContextAuth(c)
	// No caller identity is an authentication failure, not an employee with no payslips: answering an empty list would look like "you were never paid".
	if userID == 0 {
		httpx.RespondError(c, apperr.Unauthorized("Not authenticated"))
		return
	}

	slips, err := h.PayrollRun.MySlips(userID)
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

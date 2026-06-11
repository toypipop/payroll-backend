package payroll

import (
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

func GetMySlips(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var transactions []models.PayrollTransaction
	if err := config.DB.
		Joins("JOIN payroll_cycle ON payroll_cycle.id = payroll_transactions.cycle_id AND payroll_cycle.is_closed = true AND payroll_cycle.deleted_at = 0").
		Where("payroll_transactions.employees_id = ? AND payroll_transactions.deleted_at = 0", userID).
		Preload("Details").
		Order("payroll_transactions.id DESC").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type slipRow struct {
		models.PayrollTransaction
		CycleName string `json:"cycle_name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}

	cycleIDs := make([]uint, 0, len(transactions))
	for _, t := range transactions {
		cycleIDs = append(cycleIDs, t.CycleID)
	}

	var cycles []models.PayrollCycle
	if len(cycleIDs) > 0 {
		config.DB.Where("id IN ?", cycleIDs).Find(&cycles)
	}
	cycleMap := make(map[uint]models.PayrollCycle, len(cycles))
	for _, cy := range cycles {
		cycleMap[cy.ID] = cy
	}

	result := make([]slipRow, 0, len(transactions))
	for _, t := range transactions {
		cy := cycleMap[t.CycleID]
		row := slipRow{PayrollTransaction: t, CycleName: cy.CycleName}
		if !cy.StartDate.IsZero() {
			row.StartDate = cy.StartDate.Format("2006-01-02")
		}
		if !cy.EndDate.IsZero() {
			row.EndDate = cy.EndDate.Format("2006-01-02")
		}
		result = append(result, row)
	}

	c.JSON(http.StatusOK, result)
}

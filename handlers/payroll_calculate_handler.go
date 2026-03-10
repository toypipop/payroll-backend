package handlers

import (
	"math"
	"net/http"
	"payroll-backend/config"
	"payroll-backend/models"
	"payroll-backend/repository"

	"github.com/gin-gonic/gin"
)

// calculateThaiProgressiveTax computes annual income tax using Thailand's progressive brackets
func calculateThaiProgressiveTax(annualNetIncome float64) float64 {
	if annualNetIncome <= 0 {
		return 0
	}

	type bracket struct {
		max  float64
		rate float64
	}

	brackets := []bracket{
		{150000, 0.00},
		{300000, 0.05},
		{500000, 0.10},
		{750000, 0.15},
		{1000000, 0.20},
		{2000000, 0.25},
		{5000000, 0.30},
		{math.MaxFloat64, 0.35},
	}

	tax := 0.0
	prev := 0.0
	for _, b := range brackets {
		if annualNetIncome <= prev {
			break
		}
		taxable := math.Min(annualNetIncome, b.max) - prev
		tax += taxable * b.rate
		prev = b.max
	}
	return tax
}

// CalculatePayroll runs the core payroll engine for all active employees in a given cycle
func CalculatePayroll(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	cycleID := c.Param("cycle_id")

	// 1. Fetch & validate cycle - must belong to user's company and not be closed
	var cycle models.PayrollCycle
	if err := config.DB.Scopes(repository.ScopeCompanies(currentUser.CompaniesID)).
		First(&cycle, cycleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payroll cycle not found"})
		return
	}
	if cycle.IsClosed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot recalculate a closed cycle"})
		return
	}

	processedCount, skippedCount, snapshotErr := runPayrollSnapshot(currentUser.CompaniesID, cycle)

	status := http.StatusOK
	if snapshotErr != nil {
		status = http.StatusInternalServerError
	}

	c.JSON(status, gin.H{
		"message":         "Payroll calculation completed",
		"cycle_id":        cycle.ID,
		"processed_count": processedCount,
		"skipped_count":   skippedCount,
		"error": func() string {
			if snapshotErr != nil {
				return snapshotErr.Error()
			}
			return ""
		}(),
	})
}

// GetPayrollTransactions returns a summary table of all transactions for a given cycle
func GetPayrollTransactions(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	cycleID := c.Param("cycle_id")

	// Verify the cycle belongs to the user's company
	var cycle models.PayrollCycle
	if err := config.DB.Scopes(repository.ScopeCompanies(currentUser.CompaniesID)).
		First(&cycle, cycleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cycle not found"})
		return
	}

	var transactions []models.PayrollTransaction
	if err := config.DB.Where("cycle_id = ?", cycle.ID).
		Preload("Employees").
		Preload("Employees.Department").
		Preload("Employees.Position").
		Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cycle": cycle, "transactions": transactions})
}

// GetPayrollSlip returns transaction + all detail lines for a single employee payslip
func GetPayrollSlip(c *gin.Context) {
	userID, _ := repository.GetContextAuth(c)

	var currentUser models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current user not found"})
		return
	}

	transactionID := c.Param("transaction_id")

	var transaction models.PayrollTransaction
	if err := config.DB.
		Preload("Employees").
		Preload("Employees.Department").
		Preload("Employees.Position").
		Preload("Details").
		First(&transaction, transactionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	// Verify the transaction's cycle belongs to the user's company (authorization check)
	var cycle models.PayrollCycle
	if err := config.DB.Scopes(repository.ScopeCompanies(currentUser.CompaniesID)).
		First(&cycle, transaction.CycleID).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cycle": cycle, "slip": transaction})
}

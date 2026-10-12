package service

import (
	"math"
	"payroll-backend/apperr"
)

func validateSalaryAmount(amount float64, minimum float64) error {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < minimum || amount > maximumAdjustmentAmount || !hasTwoDecimals(amount) {
		return apperr.Validation("Salary is outside the supported decimal(15,2) range")
	}
	return nil
}

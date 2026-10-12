package service

import (
	"math"

	"payroll-backend/apperr"
)

// Both employee adjustment tables store decimal(15,2): thirteen integer digits.
const maximumEmployeeAdjustmentAmount = 9_999_999_999_999.99

var ErrEmployeeAdjustmentAmount = apperr.Validation("Amount must be between 0.01 and 9999999999999.99")

func validateEmployeeAdjustmentAmount(amount float64) error {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0.01 || amount > maximumEmployeeAdjustmentAmount {
		return ErrEmployeeAdjustmentAmount
	}
	return nil
}

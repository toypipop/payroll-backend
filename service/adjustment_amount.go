package service

import (
	"math"

	"payroll-backend/apperr"
)

// Employee and cycle adjustment tables store decimal(15,2): thirteen integer digits.
const maximumAdjustmentAmount = 9_999_999_999_999.99

var ErrAdjustmentAmount = apperr.Validation("Amount must be between 0.01 and 9999999999999.99")

func validateAdjustmentAmount(amount float64) error {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0.01 || amount > maximumAdjustmentAmount {
		return ErrAdjustmentAmount
	}
	return nil
}

var ErrAdjustmentCycleImmutable = apperr.Validation("Payroll cycle cannot be changed when updating an existing adjustment")

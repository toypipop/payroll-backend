package service

import (
	"fmt"
	"math"
	"strings"

	"payroll-backend/apperr"
)

// Configuration money/rate columns use decimal(10,2).
const maximumConfigAmount = 99_999_999.99

func validateConfigNumber(field string, value, maximum float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maximum {
		return apperr.Validation(fmt.Sprintf("%s must be between 0 and %.2f", field, maximum))
	}
	return nil
}

func normalizeTaxCalculation(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "progressive":
		return "Progressive", nil
	case "none", "no tax":
		return "None", nil
	default:
		return "", apperr.Validation("Tax calculation must be Progressive or None")
	}
}

package service

import (
	"math"
	"strings"
)

// Thai personal-income-tax constants applied to an annualised figure.
const (
	workExpenseRate   = 0.5
	workExpenseCap    = 100000.0
	personalAllowance = 60000.0
	monthsPerYear     = 12
)

// thaiTaxBracket is one band of the progressive scale.
type thaiTaxBracket struct {
	upTo float64
	rate float64
}

// thaiTaxBrackets is Thailand's progressive personal income tax scale.
var thaiTaxBrackets = []thaiTaxBracket{
	{150000, 0.00},
	{300000, 0.05},
	{500000, 0.10},
	{750000, 0.15},
	{1000000, 0.20},
	{2000000, 0.25},
	{5000000, 0.30},
	{math.MaxFloat64, 0.35},
}

// AnnualThaiProgressiveTax computes annual income tax from annual net income.
func AnnualThaiProgressiveTax(annualNetIncome float64) float64 {
	if annualNetIncome <= 0 {
		return 0
	}

	tax := 0.0
	previous := 0.0
	for _, bracket := range thaiTaxBrackets {
		if annualNetIncome <= previous {
			break
		}
		taxable := math.Min(annualNetIncome, bracket.upTo) - previous
		tax += taxable * bracket.rate
		previous = bracket.upTo
	}
	return tax
}

// MonthlyTax turns a monthly taxable base into one month's tax under the company's
// configured method. Anything other than an explicit "none" is treated as
// progressive, which is how unknown values have always been handled.
func MonthlyTax(taxMethod string, taxableBase float64, taxDeductible float64) float64 {
	method := strings.ToLower(strings.TrimSpace(taxMethod))
	if method == "none" || method == "no tax" {
		return 0.0
	}

	annualTaxableBase := (taxableBase - taxDeductible) * monthsPerYear
	workExpense := math.Min(annualTaxableBase*workExpenseRate, workExpenseCap)
	annualNetIncome := annualTaxableBase - workExpense - personalAllowance
	return math.Round(AnnualThaiProgressiveTax(annualNetIncome)/monthsPerYear*100) / 100
}

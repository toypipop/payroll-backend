package service

import (
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"payroll-backend/apperr"
)

// PostgreSQL varchar counts Unicode code points, not UTF-8 bytes.
// A maximum of zero denotes an unbounded text column.
func validateText(field, value string, minimum, maximum int) error {
	length := utf8.RuneCountInString(value)
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return apperr.Validation(field + " contains unsupported characters")
	}
	if length < minimum {
		return apperr.Validation(fmt.Sprintf("%s must have at least %d characters", field, minimum))
	}
	if maximum > 0 && length > maximum {
		return apperr.Validation(fmt.Sprintf("%s must have at most %d characters", field, maximum))
	}
	return nil
}

// Match JavaScript String.trim/Zod.trim, including BOM but excluding NEL.
func trimText(value string) string {
	return strings.TrimFunc(value, func(r rune) bool { return r == '\ufeff' || (r != '\u0085' && unicode.IsSpace(r)) })
}

// Ignore binary floating-point noise, but reject meaningful sub-cent values
// instead of letting PostgreSQL silently change them on write.
func hasTwoDecimals(value float64) bool {
	scaled := value * 100
	return !math.IsNaN(scaled) && !math.IsInf(scaled, 0) && math.Abs(scaled-math.Round(scaled)) < 0.000001
}

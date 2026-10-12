package service

import (
	"errors"
	"math"
	"payroll-backend/apperr"
	"testing"
)

func TestBulkSalaryOverflowBlocksEveryWrite(t *testing.T) {
	repo, employees := bulkFixture()
	repo.perEmployee[5][0].BaseSalaries = maximumAdjustmentAmount
	input := BulkRaiseInput{Mode: BulkRaiseAmount, Value: 1, EffectiveDate: "2029-01-01", Reason: "Annual adjustment", Targets: bulkTargets(5, 6)}
	svc := NewSalariesStructureService(repo, employees)
	preview, err := svc.BulkRaisePlan(salaryScope(), input)
	if err != nil || preview.BlockedCount != 1 || repo.writtenCount != 0 {
		t.Fatalf("preview: %v blocked=%d writes=%d", err, preview.BlockedCount, repo.writtenCount)
	}
	_, err = svc.BulkRaiseApply(salaryScope(), input)
	if !errors.Is(err, ErrBulkBlocked) || repo.writtenCount != 0 {
		t.Fatalf("batch partially written: %v writes=%d", err, repo.writtenCount)
	}
}

func TestBulkRaiseInvalidValuesFailBeforeLookup(t *testing.T) {
	for _, value := range []float64{math.Inf(1), math.NaN(), 1e13, 0, -1, 0.001} {
		input := BulkRaiseInput{Mode: BulkRaisePercent, Value: value, Reason: "Reason", Targets: bulkTargets(5)}
		if _, err := NewSalariesStructureService(nil, nil).BulkRaisePlan(salaryScope(), input); apperr.KindOf(err) != apperr.KindValidation {
			t.Fatalf("value=%v err=%v", value, err)
		}
	}
}

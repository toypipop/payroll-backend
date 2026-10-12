package service

import (
	"errors"
	"math"
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
)

func TestCycleAdjustmentAmountAndCycleValidation(t *testing.T) {
	cases := []struct {
		name   string
		amount float64
		cycle  uint
		want   error
	}{
		{"negative", -500, 4, ErrAdjustmentAmount}, {"zero", 0, 4, ErrAdjustmentAmount}, {"below minimum", 0.0099, 4, ErrAdjustmentAmount},
		{"above maximum", math.Nextafter(maximumAdjustmentAmount, math.Inf(1)), 4, ErrAdjustmentAmount},
		{"overflow", 1e13, 4, ErrAdjustmentAmount}, {"NaN", math.NaN(), 4, ErrAdjustmentAmount}, {"infinity", math.Inf(1), 4, ErrAdjustmentAmount},
		{"minimum", 0.01, 4, nil}, {"maximum", maximumAdjustmentAmount, 4, nil}, {"ordinary", 123.45, 4, nil},
	}
	scope := repository.AccessScope{User: models.Employees{ID: 1}, AuthorizedCompanyIDs: []uint{7}, DataScope: models.ScopeCompanies}
	for _, kind := range []string{"earning", "deduction"} {
		for _, update := range []bool{false, true} {
			checks := append([]struct {
				name   string
				amount float64
				cycle  uint
				want   error
			}{}, cases...)
			if update {
				checks = append(checks, struct {
					name   string
					amount float64
					cycle  uint
					want   error
				}{"other cycle", 100, 8, ErrAdjustmentCycleImmutable}, struct {
					name   string
					amount float64
					cycle  uint
					want   error
				}{"missing cycle", 100, 0, ErrAdjustmentCycleImmutable})
			}
			path := "create"
			if update {
				path = "update"
			}
			for _, tc := range checks {
				t.Run(kind+"/"+path+"/"+tc.name, func(t *testing.T) {
					cycles := newAdjustmentSalaryRepo(t)
					cycles.salary = models.SalariesStructure{EmployeesID: 9, IsActive: true, EffectiveDate: adjustmentDate(1)}
					employees := adjustmentEmployees{companyID: 7}
					var err error
					writes := 0
					if kind == "earning" {
						repo := &adjustmentEarningRepo{cycles: cycles}
						svc := NewCycleEarningService(repo, cycles, employees, adjustmentEarningTypes{})
						input := models.PayrollCycleEarning{CycleID: tc.cycle, CompaniesID: 7, EmployeesID: 9, EarningTypeID: 3, Amount: tc.amount}
						if update {
							_, err = svc.Update(scope, 1, input)
						} else {
							_, err = svc.Create(scope, input)
						}
						writes = repo.writes
					} else {
						repo := &adjustmentDeductionRepo{cycles: cycles}
						svc := NewCycleDeductionService(repo, cycles, employees, adjustmentDeductionTypes{})
						input := models.PayrollCycleDeduction{CycleID: tc.cycle, CompaniesID: 7, EmployeesID: 9, DeductionTypeID: 3, Amount: tc.amount}
						if update {
							_, err = svc.Update(scope, 1, input)
						} else {
							_, err = svc.Create(scope, input)
						}
						writes = repo.writes
					}
					if !errors.Is(err, tc.want) {
						t.Fatalf("error=%v want=%v", err, tc.want)
					}
					if tc.want != nil {
						if writes != 0 || cycles.locked || cycles.salaryReads != 0 {
							t.Fatalf("invalid request reached write/lock: writes=%d lock=%v salary=%d", writes, cycles.locked, cycles.salaryReads)
						}
					} else if writes != 1 || !cycles.locked || cycles.salaryReads != 1 {
						t.Fatalf("valid request lost transaction/salary/audit checks: writes=%d lock=%v salary=%d", writes, cycles.locked, cycles.salaryReads)
					}
				})
			}
		}
	}
}

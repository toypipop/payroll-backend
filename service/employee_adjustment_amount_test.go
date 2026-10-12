package service

import (
	"errors"
	"math"
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
)

type amountEarningRepo struct {
	repository.EmployeeEarningRepository
	item           models.EmployeeEarning
	writes, audits int
}

func (r *amountEarningRepo) FindScoped(repository.AccessScope, uint) (models.EmployeeEarning, error) {
	return models.EmployeeEarning{ID: 1, CompaniesID: 1, EmployeesID: 5, EarningTypeID: 2, Amount: 10}, nil
}
func (r *amountEarningRepo) Create(item *models.EmployeeEarning, audit *models.AuditLog) error {
	r.item = *item
	r.writes++
	if audit != nil {
		r.audits++
	}
	return nil
}
func (r *amountEarningRepo) UpdateFields(item *models.EmployeeEarning, fields map[string]any, audit *models.AuditLog) error {
	item.Amount = fields["amount"].(float64)
	return r.Create(item, audit)
}
func (r *amountEarningRepo) FindDetail(uint) (models.EmployeeEarning, error) { return r.item, nil }

type amountDeductionRepo struct {
	repository.EmployeeDeductionRepository
	item           models.EmployeeDeduction
	writes, audits int
}

func (r *amountDeductionRepo) FindScoped(repository.AccessScope, uint) (models.EmployeeDeduction, error) {
	return models.EmployeeDeduction{ID: 1, CompaniesID: 1, EmployeesID: 5, DeductionTypeID: 2, Amount: 10}, nil
}
func (r *amountDeductionRepo) Create(item *models.EmployeeDeduction, audit *models.AuditLog) error {
	r.item = *item
	r.writes++
	if audit != nil {
		r.audits++
	}
	return nil
}
func (r *amountDeductionRepo) UpdateFields(item *models.EmployeeDeduction, fields map[string]any, audit *models.AuditLog) error {
	item.Amount = fields["amount"].(float64)
	return r.Create(item, audit)
}
func (r *amountDeductionRepo) FindDetail(uint) (models.EmployeeDeduction, error) { return r.item, nil }

type amountEarningTypes struct {
	repository.EarningTypeRepository
}

func (amountEarningTypes) FindScoped(repository.AccessScope, uint) (models.EarningType, error) {
	return models.EarningType{ID: 2, CompaniesID: 1}, nil
}

type amountDeductionTypes struct {
	repository.DeductionTypeRepository
}

func (amountDeductionTypes) FindScoped(repository.AccessScope, uint) (models.DeductionType, error) {
	return models.DeductionType{ID: 2, CompaniesID: 1}, nil
}

func TestEmployeeAdjustmentsRejectInvalidAmountsBeforeWrite(t *testing.T) {
	invalid := []struct {
		name   string
		amount float64
	}{
		{"negative", -500}, {"zero or missing", 0}, {"below minimum", 0.0099},
		{"above maximum", math.Nextafter(maximumAdjustmentAmount, math.Inf(1))},
		{"decimal overflow", 1e13}, {"huge finite", math.MaxFloat64},
		{"NaN", math.NaN()}, {"positive infinity", math.Inf(1)}, {"negative infinity", math.Inf(-1)},
	}
	scope := repository.NewAccessScope(models.Employees{ID: 9}, models.ScopeCompanies, []uint{1})
	for _, tc := range invalid {
		for _, update := range []bool{false, true} {
			path := "create"
			if update {
				path = "update"
			}
			t.Run("earning/"+path+"/"+tc.name, func(t *testing.T) {
				r := &amountEarningRepo{}
				svc := NewEmployeeEarningService(r, nil, nil)
				input := models.EmployeeEarning{CompaniesID: 1, EmployeesID: 5, EarningTypeID: 2, Amount: tc.amount}
				var err error
				if update {
					_, err = svc.Update(scope, 1, input)
				} else {
					_, err = svc.Create(scope, input)
				}
				if !errors.Is(err, ErrAdjustmentAmount) || r.writes != 0 || r.audits != 0 {
					t.Fatalf("invalid amount wrote: err=%v writes=%d audits=%d", err, r.writes, r.audits)
				}
			})
			t.Run("deduction/"+path+"/"+tc.name, func(t *testing.T) {
				r := &amountDeductionRepo{}
				svc := NewEmployeeDeductionService(r, nil, nil)
				input := models.EmployeeDeduction{CompaniesID: 1, EmployeesID: 5, DeductionTypeID: 2, Amount: tc.amount}
				var err error
				if update {
					_, err = svc.Update(scope, 1, input)
				} else {
					_, err = svc.Create(scope, input)
				}
				if !errors.Is(err, ErrAdjustmentAmount) || r.writes != 0 || r.audits != 0 {
					t.Fatalf("invalid amount wrote: err=%v writes=%d audits=%d", err, r.writes, r.audits)
				}
			})
		}
	}
}
func TestEmployeeAdjustmentsAcceptBoundariesWithoutChangingAmounts(t *testing.T) {
	scope := repository.NewAccessScope(models.Employees{ID: 9}, models.ScopeCompanies, []uint{1})
	employees := &fakeSalaryEmployees{employee: models.Employees{ID: 5, CompaniesID: 1, IsActive: true}}
	for _, amount := range []float64{0.01, 500, 123.45, maximumAdjustmentAmount} {
		for _, update := range []bool{false, true} {
			r := &amountEarningRepo{}
			earning := NewEmployeeEarningService(r, employees, amountEarningTypes{})
			input := models.EmployeeEarning{CompaniesID: 1, EmployeesID: 5, EarningTypeID: 2, Amount: amount}
			var saved models.EmployeeEarning
			var err error
			if update {
				saved, err = earning.Update(scope, 1, input)
			} else {
				saved, err = earning.Create(scope, input)
			}
			if err != nil || saved.Amount != amount || r.writes != 1 || r.audits != 1 {
				t.Fatalf("earning amount=%v update=%v err=%v saved=%v writes=%d audits=%d", amount, update, err, saved.Amount, r.writes, r.audits)
			}
			d := &amountDeductionRepo{}
			deduction := NewEmployeeDeductionService(d, employees, amountDeductionTypes{})
			di := models.EmployeeDeduction{CompaniesID: 1, EmployeesID: 5, DeductionTypeID: 2, Amount: amount}
			var ds models.EmployeeDeduction
			if update {
				ds, err = deduction.Update(scope, 1, di)
			} else {
				ds, err = deduction.Create(scope, di)
			}
			if err != nil || ds.Amount != amount || d.writes != 1 || d.audits != 1 {
				t.Fatalf("deduction amount=%v update=%v err=%v saved=%v writes=%d audits=%d", amount, update, err, ds.Amount, d.writes, d.audits)
			}
		}
	}
}

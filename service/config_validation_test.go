package service

import (
	"math"
	"testing"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

type configPayrollRepo struct {
	repository.PayrollConfigRepository
	writes int
	audit  *models.AuditLog
	saved  models.PayrollConfig
}

func (r *configPayrollRepo) UpsertForCompany(c *models.PayrollConfig, a *models.AuditLog) error {
	r.writes++
	r.audit = a
	r.saved = *c
	return nil
}

type configAttendanceRepo struct {
	repository.AttendanceConfigRepository
	writes int
	audit  *models.AuditLog
	saved  models.AttendanceConfig
}

func (r *configAttendanceRepo) UpsertForCompany(c *models.AttendanceConfig, a *models.AuditLog) error {
	r.writes++
	r.audit = a
	r.saved = *c
	return nil
}
func configTestScope() repository.AccessScope {
	return repository.NewAccessScope(models.Employees{ID: 9}, models.ScopeCompanies, []uint{1})
}
func TestPayrollConfigurationRejectsInvalidValuesBeforeWrite(t *testing.T) {
	for _, field := range []string{"rate", "maximum", "minimum"} {
		for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1), 1e8} {
			t.Run(field, func(t *testing.T) {
				repo := &configPayrollRepo{}
				input := PayrollConfigInput{CompaniesID: 1, SsoRate: 5, SsoMaxAmount: 750, SsoMinSalary: 1650, TaxCalculation: "Progressive"}
				switch field {
				case "rate":
					input.SsoRate = value
				case "maximum":
					input.SsoMaxAmount = value
				case "minimum":
					input.SsoMinSalary = value
				}
				_, err := NewPayrollConfigService(repo).Save(configTestScope(), input)
				if apperr.KindOf(err) != apperr.KindValidation || repo.writes != 0 || repo.audit != nil {
					t.Fatalf("bad config saved: err=%v writes=%d", err, repo.writes)
				}
			})
		}
	}
	for _, method := range []string{"", "Unknown", "None;DROP TABLE"} {
		repo := &configPayrollRepo{}
		_, err := NewPayrollConfigService(repo).Save(configTestScope(), PayrollConfigInput{CompaniesID: 1, TaxCalculation: method})
		if apperr.KindOf(err) != apperr.KindValidation || repo.writes != 0 {
			t.Fatalf("unknown tax method saved: %v", err)
		}
	}
	repo := &configPayrollRepo{}
	_, err := NewPayrollConfigService(repo).Save(configTestScope(), PayrollConfigInput{CompaniesID: 1, SsoRate: 100.01, TaxCalculation: "None"})
	if apperr.KindOf(err) != apperr.KindValidation || repo.writes != 0 {
		t.Fatal("percentage above 100 accepted")
	}
}
func TestPayrollConfigurationAcceptsZeroBoundsAndSupportedTaxAliases(t *testing.T) {
	for _, tc := range []struct {
		method, want string
		rate, money  float64
	}{{"None", "None", 0, 0}, {" progressive ", "Progressive", 100, maximumConfigAmount}, {"no tax", "None", 5, 750}} {
		repo := &configPayrollRepo{}
		saved, err := NewPayrollConfigService(repo).Save(configTestScope(), PayrollConfigInput{CompaniesID: 1, SsoRate: tc.rate, SsoMaxAmount: tc.money, SsoMinSalary: tc.money, TaxCalculation: tc.method})
		if err != nil || repo.writes != 1 || repo.audit == nil || saved.SsoRate != tc.rate || saved.SsoMaxAmount != tc.money || saved.SsoMinSalary != tc.money || saved.TaxCalculation != tc.want {
			t.Fatalf("valid config lost: %+v err=%v", saved, err)
		}
	}
}
func TestAttendanceConfigurationRejectsInvalidRatesBeforeWrite(t *testing.T) {
	for _, field := range []string{"late", "absent", "overtime", "extra"} {
		for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1), 1e8} {
			t.Run(field, func(t *testing.T) {
				repo := &configAttendanceRepo{}
				input := AttendanceConfigInput{CompaniesID: 1}
				switch field {
				case "late":
					input.LateRatePerMinute = value
				case "absent":
					input.AbsentRatePerDay = value
				case "overtime":
					input.OvertimeRatePerMinute = value
				case "extra":
					input.ExtraRatePerDay = value
				}
				_, err := NewAttendanceConfigService(repo).Save(configTestScope(), input)
				if apperr.KindOf(err) != apperr.KindValidation || repo.writes != 0 || repo.audit != nil {
					t.Fatalf("invalid rates saved: %v writes=%d", err, repo.writes)
				}
			})
		}
	}
	for _, value := range []float64{0, maximumConfigAmount} {
		repo := &configAttendanceRepo{}
		input := AttendanceConfigInput{CompaniesID: 1, LateRatePerMinute: value, AbsentRatePerDay: value, OvertimeRatePerMinute: value, ExtraRatePerDay: value}
		saved, err := NewAttendanceConfigService(repo).Save(configTestScope(), input)
		if err != nil || repo.writes != 1 || repo.audit == nil || saved.LateRatePerMinute != value || saved.AbsentRatePerDay != value || saved.OvertimeRatePerMinute != value || saved.ExtraRatePerDay != value || saved.IsLateDeductionEnabled || saved.IsAbsentDeductionEnabled {
			t.Fatalf("zero/boundary config lost: %+v err=%v", saved, err)
		}
	}
}

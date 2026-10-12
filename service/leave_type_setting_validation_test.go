package service

import (
	"math"
	"strings"
	"testing"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

type leaveTypeValidationRepo struct {
	repository.LeaveTypeSettingRepository
	writes int
	saved  models.LeaveTypeSetting
}

func TestLeaveTypeNameValidation(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, tc := range []struct {
			name, want string
			valid      bool
		}{
			{" ลาป่วย ", "ลาป่วย", true},
			{strings.Repeat("ก", 100), strings.Repeat("ก", 100), true},
			{strings.Repeat("😀", 100), strings.Repeat("😀", 100), true},
			{"", "", false}, {" \t\n", "", false},
			{strings.Repeat("ก", 101), "", false},
			{strings.Repeat("😀", 101), "", false},
		} {
			repo := &leaveTypeValidationRepo{}
			svc := NewLeaveTypeSettingService(repo)
			input := models.LeaveTypeSetting{CompaniesID: 1, Name: tc.name}
			var err error
			if update {
				_, err = svc.Update(configTestScope(), 1, input)
			} else {
				_, err = svc.Create(configTestScope(), input)
			}
			if tc.valid {
				if err != nil || repo.writes != 1 || repo.saved.Name != tc.want {
					t.Fatalf("name not preserved: update=%v err=%v saved=%q", update, err, repo.saved.Name)
				}
			} else if apperr.KindOf(err) != apperr.KindValidation || repo.writes != 0 {
				t.Fatalf("invalid name persisted: update=%v err=%v", update, err)
			}
		}
	}
}

func (r *leaveTypeValidationRepo) FindScoped(repository.AccessScope, uint) (models.LeaveTypeSetting, error) {
	return models.LeaveTypeSetting{ID: 1, CompaniesID: 1, Name: "Annual"}, nil
}
func (r *leaveTypeValidationRepo) Create(row *models.LeaveTypeSetting, _ *models.AuditLog) error {
	r.writes++
	r.saved = *row
	return nil
}
func (r *leaveTypeValidationRepo) Save(row *models.LeaveTypeSetting, audit *models.AuditLog) error {
	return r.Create(row, audit)
}

func TestLeaveTypeAnnualLimitValidation(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, days := range []float64{-0.01, 1000, 999.991, math.NaN(), math.Inf(1), math.Inf(-1), 0, 0.01, 30, 999.99} {
			repo := &leaveTypeValidationRepo{}
			svc := NewLeaveTypeSettingService(repo)
			input := models.LeaveTypeSetting{CompaniesID: 1, Name: "Annual", MaxDaysPerYear: days}
			var err error
			if update {
				_, err = svc.Update(configTestScope(), 1, input)
			} else {
				_, err = svc.Create(configTestScope(), input)
			}
			valid := !math.IsNaN(days) && !math.IsInf(days, 0) && days >= 0 && days <= 999.99
			if valid {
				if err != nil || repo.writes != 1 || repo.saved.MaxDaysPerYear != days {
					t.Fatalf("update=%v days=%v: err=%v writes=%d", update, days, err, repo.writes)
				}
			} else if apperr.KindOf(err) != apperr.KindValidation || repo.writes != 0 {
				t.Fatalf("invalid limit persisted: update=%v days=%v err=%v writes=%d", update, days, err, repo.writes)
			}
		}
	}
}

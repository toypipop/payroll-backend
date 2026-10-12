package service

import (
	"math"
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

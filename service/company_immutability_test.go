package service

import (
	"errors"
	"payroll-backend/models"
	"payroll-backend/repository"
	"testing"
)

type immutableEarningRepo struct {
	repository.EmployeeEarningRepository
}

func (immutableEarningRepo) FindScoped(repository.AccessScope, uint) (models.EmployeeEarning, error) {
	return models.EmployeeEarning{ID: 1, CompaniesID: 1}, nil
}

type immutableDeductionRepo struct {
	repository.EmployeeDeductionRepository
}

func (immutableDeductionRepo) FindScoped(repository.AccessScope, uint) (models.EmployeeDeduction, error) {
	return models.EmployeeDeduction{ID: 1, CompaniesID: 1}, nil
}

type immutableLeaveRepo struct {
	repository.LeaveRequestRepository
}

func (immutableLeaveRepo) FindScoped(repository.AccessScope, uint) (models.LeaveRequest, error) {
	return models.LeaveRequest{ID: 1, CompaniesID: 1, Status: models.LeaveStatusPending}, nil
}

type immutableTimeRepo struct{ repository.TimeLogRepository }

func (immutableTimeRepo) FindScoped(repository.AccessScope, uint) (models.TimeLog, error) {
	return models.TimeLog{ID: 1, CompaniesID: 1}, nil
}
func TestUpdatesCannotMoveCompanyEvenWithAccessToBoth(t *testing.T) {
	scope := repository.NewAccessScope(models.Employees{ID: 9}, models.ScopeCompanies, []uint{1, 2})
	cases := []struct {
		name string
		run  func() error
	}{
		{"employee earning", func() error {
			_, err := NewEmployeeEarningService(immutableEarningRepo{}, nil, nil).Update(scope, 1, models.EmployeeEarning{CompaniesID: 2})
			return err
		}},
		{"employee deduction", func() error {
			_, err := NewEmployeeDeductionService(immutableDeductionRepo{}, nil, nil).Update(scope, 1, models.EmployeeDeduction{CompaniesID: 2})
			return err
		}},
		{"leave", func() error {
			_, err := NewLeaveRequestService(immutableLeaveRepo{}, nil, nil).Update(scope, 1, LeaveInput{CompaniesID: 2})
			return err
		}},
		{"time log", func() error {
			_, err := NewTimeLogService(immutableTimeRepo{}, nil, nil).Update(scope, 1, TimeLogInput{CompaniesID: 2})
			return err
		}},
		{"company access", func() error {
			_, err := NewEmployeeCompanyAccessService(&scopedAccessRepo{}, nil, nil).Update(scope, 3, EmployeeCompanyAccessInput{CompanyID: 2})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, ErrCompanyImmutable) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

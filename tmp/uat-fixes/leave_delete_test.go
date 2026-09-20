package uatfixes_test

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"
	"testing"
)

type approvedLeaveRepo struct {
	repository.LeaveRequestRepository
}

func (approvedLeaveRepo) FindScoped(repository.AccessScope, uint) (models.LeaveRequest, error) {
	return models.LeaveRequest{Status: models.LeaveStatusApproved}, nil
}

func TestApprovedLeaveCannotBeDeleted(t *testing.T) {
	// Any call to the embedded, nil Delete method panics: no mutation is permitted.
	s := service.NewLeaveRequestService(approvedLeaveRepo{}, nil, nil)
	if err := s.Delete(repository.AccessScope{}, 1); apperr.KindOf(err) != apperr.KindConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

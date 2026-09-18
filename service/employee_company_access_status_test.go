package service

import (
	"errors"
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
)

type inactiveCompanyAccessRepo struct {
	repository.EmployeeCompanyAccessRepository
}

func (inactiveCompanyAccessRepo) FindEmployeeBrief(uint) (models.Employees, error) {
	return models.Employees{ID: 9, CompaniesID: 7, IsActive: false}, nil
}

func (inactiveCompanyAccessRepo) FindScoped(repository.AccessScope, uint) (models.EmployeeCompanyAccess, error) {
	return models.EmployeeCompanyAccess{ID: 1, EmployeeID: 9, CompanyID: 7}, nil
}

func TestInactiveEmployeeCannotReceiveCompanyAccess(t *testing.T) {
	svc := NewEmployeeCompanyAccessService(inactiveCompanyAccessRepo{}, nil)
	scope := repository.AccessScope{AuthorizedCompanyIDs: []uint{7}, DataScope: models.ScopeCompanies}
	input := EmployeeCompanyAccessInput{EmployeeID: 9, CompanyID: 7, RoleID: 2, IsActive: true}
	if _, err := svc.Create(scope, input); !errors.Is(err, ErrEmployeeInactive) {
		t.Fatalf("create error=%v, want inactive employee", err)
	}
	if _, err := svc.Update(scope, 1, input); !errors.Is(err, ErrEmployeeInactive) {
		t.Fatalf("update error=%v, want inactive employee", err)
	}
}

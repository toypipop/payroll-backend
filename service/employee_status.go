package service

import (
	"payroll-backend/models"
	"payroll-backend/repository"
)

var ErrEmployeeInactive = repository.ErrEmployeeInactive

func requireActiveEmployee(employee models.Employees) error {
	if !employee.IsActive {
		return ErrEmployeeInactive
	}
	return nil
}

func activeEmployeeByID(repo repository.EmployeesRepository, scope repository.AccessScope, id uint) error {
	employee, err := repo.FindBriefScoped(scope, id)
	if err != nil {
		return err
	}
	return requireActiveEmployee(employee)
}

func (s *EmployeesService) Options(scope repository.AccessScope) ([]repository.EmployeeOption, error) {
	if scope.DataScope == "" {
		scope.DataScope = models.ScopeCompanies
	}
	return s.repo.ListActiveOptions(scope)
}

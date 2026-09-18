package service

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
)

var ErrCycleEmployeeNoSalary = apperr.Conflict("Employee has no enabled salary effective in this payroll cycle. Reload the employee options and select another employee.")

func (s *PayrollRunService) CycleEmployeeOptions(scope repository.AccessScope, cycleID uint) ([]repository.CycleEmployeeOption, error) {
	// Ungated dropdowns expose only identities in accessible companies; mutations
	// still use their module's employee scope. Preserve an explicitly supplied scope.
	if scope.DataScope == "" {
		scope.DataScope = models.ScopeCompanies
	}
	cycle, err := s.repo.FindCycleScoped(scope, cycleID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListCycleEmployeeOptions(scope, cycle)
}

// An enabled salary on or before cycle end covers both full-cycle and prorated pay.
// Call while holding the cycle lock, which serializes salary changes and cycle close.
func validateCycleEmployeeSalary(repo repository.PayrollRunRepository, cycle models.PayrollCycle, employeeID uint) error {
	_, err := repo.FindEarliestSalaryOnOrBefore(employeeID, cycle.EndDate)
	if errors.Is(err, repository.ErrPayrollSalaryMissing) {
		return ErrCycleEmployeeNoSalary
	}
	return err
}

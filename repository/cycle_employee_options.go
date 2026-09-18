package repository

import (
	"time"

	"payroll-backend/models"

	"gorm.io/gorm"
)

// CycleEmployeeOption deliberately excludes salary amounts and employee private data.
type CycleEmployeeOption struct {
	ID            uint   `json:"id"`
	CompaniesID   uint   `json:"companies_id"`
	EmployeesCode string `json:"employees_code"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
}

// usableSalaryBy is shared by payroll salary lookups and cycle employee options.
func usableSalaryBy(date time.Time) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("is_active = ? AND effective_date <= ?", true, date)
	}
}

func (r *gormPayrollRunRepository) ListCycleEmployeeOptions(scope AccessScope, cycle models.PayrollCycle) ([]CycleEmployeeOption, error) {
	options := make([]CycleEmployeeOption, 0)
	salaries := r.db.Model(&models.SalariesStructure{}).Select("employees_id").Scopes(usableSalaryBy(cycle.EndDate))
	err := r.db.Model(&models.Employees{}).
		Scopes(scope.ApplyEmployee("employees.companies_id", "employees.id")).
		Where("employees.companies_id = ?", cycle.CompaniesID).
		Where("employees.is_active = ?", true).
		Where("employees.id IN (?)", salaries).
		Select("employees.id, employees.companies_id, employees.employees_code, employees.first_name, employees.last_name").
		Order("employees.employees_code, employees.id").Find(&options).Error
	return options, err
}

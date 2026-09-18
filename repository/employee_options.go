package repository

import "payroll-backend/models"

type EmployeeOption struct {
	ID            uint   `json:"id"`
	CompaniesID   uint   `json:"companies_id"`
	EmployeesCode string `json:"employees_code"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	IsActive      bool   `json:"is_active"`
}

func (r *gormEmployeesRepository) ListActiveOptions(scope AccessScope) ([]EmployeeOption, error) {
	options := make([]EmployeeOption, 0)
	err := r.scoped(scope).Model(&models.Employees{}).
		Select("employees.id, employees.companies_id, employees.employees_code, employees.first_name, employees.last_name, employees.is_active").
		Where("employees.is_active = ?", true).Order("employees.employees_code, employees.id").Find(&options).Error
	return options, err
}

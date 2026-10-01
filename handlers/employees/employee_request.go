package employees

import "payroll-backend/models"

// createEmployeeRequest accepts form fields only. Password is write-only;
// IDs, timestamps and associations are assigned by the server.
type createEmployeeRequest struct {
	CompaniesID      uint   `json:"companies_id"`
	EmployeesCode    string `json:"employees_code"`
	Email            string `json:"email"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	Password         string `json:"password"`
	DeptID           uint   `json:"dept_id"`
	PosID            uint   `json:"pos_id"`
	RolesID          uint   `json:"roles_id"`
	EmploymentTypeID *uint  `json:"employment_type_id"`
	IsActive         bool   `json:"is_active"`
	BankID           *uint  `json:"bank_id"`
	AccountName      string `json:"account_name"`
	AccountNo        string `json:"account_no"`
}

func (input createEmployeeRequest) employee() models.Employees {
	return models.Employees{
		CompaniesID: input.CompaniesID, EmployeesCode: input.EmployeesCode,
		Email: input.Email, FirstName: input.FirstName, LastName: input.LastName,
		Password: input.Password, DeptID: input.DeptID, PosID: input.PosID,
		RolesID: input.RolesID, EmploymentTypeID: input.EmploymentTypeID,
		IsActive: input.IsActive, BankID: input.BankID,
		AccountName: input.AccountName, AccountNo: input.AccountNo,
	}
}

package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type Employees struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Use Composite Unique Index with DeletedAt for both EmployeesCode and Email
	// Set default:0 to ensure unique index works correctly with soft delete (English Comment)
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_code_companies_deleted;uniqueIndex:idx_email_companies_deleted;not null;default:0" json:"-"`

	// Primary/home tenant. App access is stored in EmployeeCompanyAccess.
	CompaniesID uint `gorm:"uniqueIndex:idx_emp_code_companies_deleted;uniqueIndex:idx_email_companies_deleted;not null" json:"companies_id"`

	EmployeesCode string `gorm:"uniqueIndex:idx_emp_code_companies_deleted;not null;size:100" json:"employees_code"`
	Email         string `gorm:"uniqueIndex:idx_email_companies_deleted;not null;size:150" json:"email"`

	FirstName string `gorm:"not null" json:"first_name"`
	LastName  string `gorm:"not null" json:"last_name"`
	Password  string `gorm:"not null" json:"password,omitempty"`

	DeptID  uint `json:"dept_id"`
	PosID   uint `json:"pos_id"`
	RolesID uint `gorm:"not null" json:"roles_id"`

	EmploymentTypeID *uint `json:"employment_type_id"` // FK -> EmploymentType

	IsActive bool `gorm:"default:true" json:"is_active"`

	// Bank account fields (merged from employee_bank_account table)
	BankID      *uint  `json:"bank_id"` // FK -> Bank; BankName is resolved live via the Bank relation
	AccountName string `gorm:"size:200" json:"account_name"`
	AccountNo   string `gorm:"size:50" json:"account_no"`

	// Relations
	Companies         Companies               `gorm:"foreignKey:CompaniesID" json:"companies"`
	CompanyAccess     []EmployeeCompanyAccess `gorm:"foreignKey:EmployeeID" json:"company_access,omitempty"`
	Department        Department              `gorm:"foreignKey:DeptID" json:"department"`
	Position          Position                `gorm:"foreignKey:PosID" json:"position"`
	Roles             Roles                   `gorm:"foreignKey:RolesID" json:"roles"`
	SalariesStructure SalariesStructure       `gorm:"foreignKey:EmployeesID" json:"salaries_structure"`
	EmploymentType    *EmploymentType         `gorm:"foreignKey:EmploymentTypeID" json:"employment_type"`
	Bank              *Bank                   `gorm:"foreignKey:BankID" json:"bank"`
}

// SalariesStructure defines the base pay for each Employees
type SalariesStructure struct {
	ID            uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	DeletedAt     soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_sal_eff_deleted;not null;default:0" json:"deleted_at"`
	CompaniesID   uint                  `gorm:"not null;index" json:"companies_id"`
	EmployeesID   uint                  `gorm:"not null;index;uniqueIndex:idx_emp_sal_eff_deleted" json:"employees_id"`
	BaseSalaries  float64               `gorm:"type:decimal(15,2)" json:"base_salaries"`
	EffectiveDate time.Time             `gorm:"not null;uniqueIndex:idx_emp_sal_eff_deleted" json:"effective_date"`
	Companies     Companies             `gorm:"foreignKey:CompaniesID" json:"companies"`
	Employees     *Employees            `gorm:"foreignKey:EmployeesID" json:"employees,omitempty"`
}

// EmployeeEarning assigns an additional earning type + fixed monthly amount to an employee
type EmployeeEarning struct {
	ID            uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	DeletedAt     soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_etype_deleted;not null;default:0" json:"-"`
	CompaniesID   uint                  `gorm:"not null;index" json:"companies_id"`
	EmployeesID   uint                  `gorm:"uniqueIndex:idx_emp_etype_deleted;not null;index" json:"employees_id"`
	EarningTypeID uint                  `gorm:"uniqueIndex:idx_emp_etype_deleted;not null" json:"earning_type_id"`
	Amount        float64               `gorm:"type:decimal(15,2);not null" json:"amount"`
	EarningType   EarningType           `gorm:"foreignKey:EarningTypeID" json:"earning_type"`
	Employees     Employees             `gorm:"foreignKey:EmployeesID" json:"employees"`
}

// EmployeeDeduction assigns a deduction type + fixed monthly amount to an employee
type EmployeeDeduction struct {
	ID              uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	DeletedAt       soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_dtype_deleted;not null;default:0" json:"-"`
	CompaniesID     uint                  `gorm:"not null;index" json:"companies_id"`
	EmployeesID     uint                  `gorm:"uniqueIndex:idx_emp_dtype_deleted;not null;index" json:"employees_id"`
	DeductionTypeID uint                  `gorm:"uniqueIndex:idx_emp_dtype_deleted;not null" json:"deduction_type_id"`
	Amount          float64               `gorm:"type:decimal(15,2);not null" json:"amount"`
	DeductionType   DeductionType         `gorm:"foreignKey:DeductionTypeID" json:"deduction_type"`
	Employees       Employees             `gorm:"foreignKey:EmployeesID" json:"employees"`
}

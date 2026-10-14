package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

type DataScope string

const (
	ScopeOwn        DataScope = "Own"
	ScopeDepartment DataScope = "Department"
	ScopeCompanies  DataScope = "Companies"
)

type ModuleKey string

const (
	ModuleDashboard  ModuleKey = "dashboard"
	ModuleEmployees  ModuleKey = "employees"
	ModuleAttendance ModuleKey = "attendance"
	ModuleSettings   ModuleKey = "settings"
)

// Roles model with fixed size
type Roles struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	DeletedAt   soft_delete.DeletedAt `gorm:"uniqueIndex:idx_roles_name_companies_deleted;not null;default:0" json:"-"`
	CompaniesID uint                  `gorm:"uniqueIndex:idx_roles_name_companies_deleted;not null" json:"companies_id"`
	RolesName   string                `gorm:"uniqueIndex:idx_roles_name_companies_deleted;not null;size:255" json:"roles_name"`
	Description string                `json:"description"`
	Companies   Companies             `gorm:"foreignKey:CompaniesID" json:"companies"`
}

type EmployeeCompanyAccess struct {
	ID         uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
	DeletedAt  soft_delete.DeletedAt `gorm:"uniqueIndex:idx_emp_company_access_deleted;not null;default:0" json:"-"`
	EmployeeID uint                  `gorm:"uniqueIndex:idx_emp_company_access_deleted;not null;index" json:"employee_id"`
	CompanyID  uint                  `gorm:"uniqueIndex:idx_emp_company_access_deleted;not null;index" json:"company_id"`
	RoleID     uint                  `gorm:"uniqueIndex:idx_emp_company_access_deleted;not null;index" json:"role_id"`
	IsActive   bool                  `gorm:"default:true;index" json:"is_active"`
	Employee   Employees             `gorm:"foreignKey:EmployeeID" json:"employee,omitempty"`
	Company    Companies             `gorm:"foreignKey:CompanyID" json:"company,omitempty"`
	Role       Roles                 `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

func (EmployeeCompanyAccess) TableName() string {
	return "employee_company_access"
}

// AppModule model with fixed size
type AppModule struct {
	ID         uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
	DeletedAt  soft_delete.DeletedAt `gorm:"index" json:"-"`
	ModuleName string                `gorm:"not null" json:"module_name"`
	ModuleKey  ModuleKey             `gorm:"unique;not null;size:100" json:"module_key" binding:"required"`
	RoutePath  string                `json:"route_path"`
	Icon       string                `json:"icon"`
	Order      int                   `json:"order"`
	ParentID   *uint                 `json:"parent_id"`
	IsCategory bool                  `gorm:"default:false" json:"is_category"`
	IsActive   bool                  `gorm:"default:true" json:"is_active"`

	// Capability columns.
	ScopeLevel     ModuleScopeLevel `gorm:"size:20;not null;default:'company'" json:"scope_level"`
	AllowedActions []string         `gorm:"serializer:json" json:"allowed_actions"`
	AllowedScopes  []string         `gorm:"serializer:json" json:"allowed_scopes"`
	ActionsAtomic  bool             `gorm:"default:false" json:"actions_atomic"`
}

// RolesModuleAccess with composite index
type RolesModuleAccess struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	DeletedAt   soft_delete.DeletedAt `gorm:"softDelete:nano;uniqueIndex:idx_roles_module_deleted;not null;default:0" json:"-"`
	CompaniesID uint                  `gorm:"uniqueIndex:idx_roles_module_deleted;not null" json:"companies_id"`
	RolesID     uint                  `gorm:"uniqueIndex:idx_roles_module_deleted;not null" json:"roles_id"`
	ModuleID    uint                  `gorm:"uniqueIndex:idx_roles_module_deleted;not null" json:"module_id"`
	CanView     bool                  `json:"can_view"`
	CanCreate   bool                  `json:"can_create"`
	CanEdit     bool                  `json:"can_edit"`
	CanDelete   bool                  `json:"can_delete"`
	DataScope   DataScope             `json:"data_scope" binding:"required,oneof=Own Department Companies"`
	AppModule   AppModule             `gorm:"foreignKey:ModuleID" json:"app_module"`
}

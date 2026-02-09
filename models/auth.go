package models

import "time"

type Role struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// Support Multi-tenancy for roles (English Comment)
	CompaniesID    uint      `gorm:"index:idx_role_companies;not null" json:"companies_id"`
	RoleName     string    `gorm:"index:idx_role_companies;not null" json:"role_name"`
	Description  string    `json:"description"`
	IsSuperAdmin bool      `gorm:"default:false" json:"is_super_admin"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AppModule struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ModuleName string `gorm:"not null" json:"module_name"`
	ModuleKey  string `gorm:"unique;not null" json:"module_key"`
	RoutePath  string `json:"route_path"`
	Icon       string `json:"icon"`
	Order      int    `json:"order"`
	// Support Nested Menus (English Comment)
	ParentID  *uint     `json:"parent_id"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type RoleModuleAccess struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	CompaniesID uint `gorm:"uniqueIndex:idx_role_module;not null" json:"companies_id"`
	RoleID    uint `gorm:"uniqueIndex:idx_role_module;not null" json:"role_id"`
	ModuleID  uint `gorm:"uniqueIndex:idx_role_module;not null" json:"module_id"`

	CanView   bool `json:"can_view"`
	CanCreate bool `json:"can_create"`
	CanEdit   bool `json:"can_edit"`
	CanDelete bool `json:"can_delete"`

	DataScope string `json:"data_scope"`

	AppModule AppModule `gorm:"foreignKey:ModuleID" json:"app_module"`
}

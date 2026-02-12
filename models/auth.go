package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// Role model with fixed size for Indexing (English Comment)
type Role struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Use Composite Unique Index with DeletedAt
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_role_name_company_deleted;not null;default:0" json:"-"`

	CompaniesID uint `gorm:"uniqueIndex:idx_role_name_company_deleted;not null" json:"companies_id"`
	// Add size:255 to RoleName to support MySQL Index (English Comment)
	RoleName     string `gorm:"uniqueIndex:idx_role_name_company_deleted;not null;size:255" json:"role_name"`
	Description  string `json:"description"`
	IsSuperAdmin bool   `gorm:"default:false" json:"is_super_admin"`
}

// AppModule model with fixed size for ModuleKey (English Comment)
type AppModule struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"index" json:"-"`

	ModuleName string `gorm:"not null" json:"module_name"`
	// Add size:100 for unique key specification (English Comment)
	ModuleKey string `gorm:"unique;not null;size:100" json:"module_key"`
	RoutePath string `json:"route_path"`
	Icon      string `json:"icon"`
	Order     int    `json:"order"`
	ParentID  *uint  `json:"parent_id"`
	IsActive  bool   `gorm:"default:true" json:"is_active"`
}

// RoleModuleAccess model with Composite Unique Index (English Comment)
type RoleModuleAccess struct {
	ID        uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	DeletedAt soft_delete.DeletedAt `gorm:"uniqueIndex:idx_role_module_deleted;not null;default:0" json:"-"`

	CompaniesID uint `gorm:"uniqueIndex:idx_role_module_deleted;not null" json:"companies_id"`
	RoleID      uint `gorm:"uniqueIndex:idx_role_module_deleted;not null" json:"role_id"`
	ModuleID    uint `gorm:"uniqueIndex:idx_role_module_deleted;not null" json:"module_id"`

	CanView   bool   `json:"can_view"`
	CanCreate bool   `json:"can_create"`
	CanEdit   bool   `json:"can_edit"`
	CanDelete bool   `json:"can_delete"`
	DataScope string `json:"data_scope"`

	AppModule AppModule `gorm:"foreignKey:ModuleID" json:"app_module"`
}

package models

import "time"

type Role struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RoleName     string    `gorm:"unique;not null" json:"role_name"`
	Description  string    `json:"description"`
	IsSuperAdmin bool      `gorm:"default:false" json:"is_super_admin"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AppModule struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ModuleName string    `gorm:"not null" json:"module_name"`
	ModuleKey  string    `gorm:"unique;not null" json:"module_key"` // e.g., "EMPLOYEE", "PAYROLL"
	RoutePath  string    `json:"route_path"`
	Icon       string    `json:"icon"`  // For Frontend menu icons (English Comment)
	Order      int       `json:"order"` // For sorting menu items (English Comment)
	CreatedAt  time.Time `json:"created_at"`
}

type RoleModuleAccess struct {
	RoleID    uint   `gorm:"primaryKey" json:"role_id"`
	ModuleID  uint   `gorm:"primaryKey" json:"module_id"`
	CanView   bool   `gorm:"default:true" json:"can_view"`
	CanEdit   bool   `gorm:"default:false" json:"can_edit"`
	DataScope string `gorm:"type:enum('ALL','DEPARTMENT','OWN');default:'OWN'" json:"data_scope"`

	// Relationships for easy preloading (English Comment)
	Role      Role      `gorm:"foreignKey:RoleID"`
	AppModule AppModule `gorm:"foreignKey:ModuleID"`
}

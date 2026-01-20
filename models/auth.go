package models

type Role struct {
	ID          uint   `gorm:"primaryKey"`
	RoleName    string `gorm:"unique;not null"`
	Description string
}

type AppModule struct {
	ID         uint   `gorm:"primaryKey"`
	ModuleName string `gorm:"not null"`
	ModuleKey  string `gorm:"unique;not null"`
	RoutePath  string
}

type RoleModuleAccess struct {
	RoleID    uint   `gorm:"primaryKey"`
	ModuleID  uint   `gorm:"primaryKey"`
	CanView   bool   `gorm:"default:true"`
	CanEdit   bool   `gorm:"default:false"`
	DataScope string `gorm:"type:enum('ALL','DEPARTMENT','OWN');default:'OWN'"`
}

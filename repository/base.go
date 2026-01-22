package repository

import "gorm.io/gorm"

func ScopeCompany(companyID uint) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("company_id = ?", companyID)
	}
}

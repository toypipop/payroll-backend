package repository

import "gorm.io/gorm"

/* --- ScopeCompany ensures data isolation between companies (English Comment) --- */
func ScopeCompany(companyID uint) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("company_id = ?", companyID)
	}
}

package repository

import "gorm.io/gorm"

func ScopeCompanies(companiesID uint, columnName ...string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		col := "companies_id"

		if len(columnName) > 0 {
			col = columnName[0]
		}

		return db.Where(col+" = ?", companiesID)
	}
}

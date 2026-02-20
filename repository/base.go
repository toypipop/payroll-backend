package repository

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

func ScopeCompanies(companiesID uint, columnName ...string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		col := "companies_id"
		if len(columnName) > 0 {
			col = columnName[0]
		}
		return db.Where(col+" = ?", companiesID)
	}
}

func ScopeDataAccess(user models.Employees, scope models.DataScope, tableCol ...string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		col := "employees_id"
		if len(tableCol) > 0 {
			col = tableCol[0]
		}

		switch scope {
		case models.ScopeOwn:
			return db.Where(col+" = ?", user.ID)

		case models.ScopeDepartment:

			subQuery := db.Session(&gorm.Session{NewDB: true}).
				Table("employees").
				Select("id").
				Where("dept_id = ? AND companies_id = ?", user.DeptID, user.CompaniesID)

			return db.Where(col+" IN (?)", subQuery)

		case models.ScopeCompany:
			return db

		case models.ScopeAll:
			return db

		default:
			return db.Where(col+" = ?", user.ID)
		}
	}
}

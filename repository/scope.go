package repository

import (
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
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

		// Skip employee filtering for company data
		if col == "unused" {
			return db
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

		case models.ScopeCompanies:
			return db

		case models.ScopeAll:
			return db

		default:
			return db.Where(col+" = ?", user.ID)
		}
	}
}

func GetContextAuth(c *gin.Context) (uint, models.DataScope) {
	valUser, _ := c.Get("user_id")
	var userID uint
	switch v := valUser.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	}

	valScope, _ := c.Get("data_scope")
	scope, _ := valScope.(models.DataScope)

	return userID, scope
}

func ApplyScopes(user models.Employees, scope models.DataScope, companiesCol string, accessCol string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		query := db.Scopes(ScopeDataAccess(user, scope, accessCol))
		if scope != models.ScopeAll {
			query = query.Scopes(ScopeCompanies(user.CompaniesID, companiesCol))
		}
		return query
	}
}

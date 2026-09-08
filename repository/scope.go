package repository

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

func ScopeCompanyIDs(companyIDs []uint, columnName ...string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		col := "companies_id"
		if len(columnName) > 0 {
			col = columnName[0]
		}
		if len(companyIDs) == 0 {
			return db.Where("1 = 0")
		}
		return db.Where(col+" IN ?", companyIDs)
	}
}

func DefaultCompanyID(user models.Employees) uint {
	companyID, _ := DefaultCompanyAccess(user)
	return companyID
}

func DefaultCompanyAccess(user models.Employees) (companyID uint, roleID uint) {
	return user.CompaniesID, user.RolesID
}

// Row-level scoping has two entry points, and picking between them is the whole statement about a table.

// applyCompanyFilter narrows to the authorized companies, denying everything when there are none.
func applyCompanyFilter(db *gorm.DB, companyIDs []uint, companiesCol string) *gorm.DB {
	if len(companyIDs) == 0 {
		return db.Where("1 = 0")
	}
	return db.Scopes(ScopeCompanyIDs(companyIDs, companiesCol))
}

// scopeDataAccess turns a DataScope into a predicate on employeeCol.
func scopeDataAccess(user models.Employees, scope models.DataScope, companyIDs []uint, employeeCol string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		switch scope {
		case models.ScopeOwn:
			return db.Where(employeeCol+" = ?", user.ID)
		case models.ScopeDepartment:
			if len(companyIDs) == 0 {
				return db.Where("1 = 0")
			}
			var userCategoryID uint
			db.Session(&gorm.Session{NewDB: true}).
				Table("department").
				Select("category_id").
				Where("id = ?", user.DeptID).
				Scan(&userCategoryID)
			if userCategoryID == 0 {
				return db.Where("1 = 0")
			}
			subQuery := db.Session(&gorm.Session{NewDB: true}).
				Table("employees").
				Select("employees.id").
				Joins("JOIN department ON department.id = employees.dept_id AND department.deleted_at = 0").
				Where("department.category_id = ? AND employees.companies_id IN ?", userCategoryID, companyIDs)
			return db.Where(employeeCol+" IN (?)", subQuery)
		case models.ScopeCompanies:
			return db
		default:
			return db.Where(employeeCol+" = ?", user.ID)
		}
	}
}

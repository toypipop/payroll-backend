package repository

import (
	"payroll-backend/config"
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

func AccessibleCompanyIDs(userID uint) []uint {
	var user models.Employees
	if err := config.DB.Select("id, companies_id").First(&user, userID).Error; err != nil {
		return nil
	}

	seen := map[uint]bool{}
	ids := make([]uint, 0)

	if user.CompaniesID != 0 {
		seen[user.CompaniesID] = true
		ids = append(ids, user.CompaniesID)
	}

	var mappings []models.EmployeeCompanyAccess
	if err := config.DB.
		Where("employee_id = ? AND is_active = ?", userID, true).
		Find(&mappings).Error; err == nil {
		for _, m := range mappings {
			if m.CompanyID != 0 && !seen[m.CompanyID] {
				seen[m.CompanyID] = true
				ids = append(ids, m.CompanyID)
			}
		}
	}
	return ids
}

func AccessibleRoleIDs(userID uint) []uint {
	var user models.Employees
	if err := config.DB.Select("id, roles_id").First(&user, userID).Error; err != nil {
		return nil
	}

	seen := map[uint]bool{}
	ids := make([]uint, 0)

	if user.RolesID != 0 {
		seen[user.RolesID] = true
		ids = append(ids, user.RolesID)
	}

	var mappings []models.EmployeeCompanyAccess
	if err := config.DB.
		Where("employee_id = ? AND is_active = ?", userID, true).
		Find(&mappings).Error; err == nil {
		for _, m := range mappings {
			if m.RoleID != 0 && !seen[m.RoleID] {
				seen[m.RoleID] = true
				ids = append(ids, m.RoleID)
			}
		}
	}
	return ids
}

func AccessibleCompanyIDsForRoles(userID uint, roleIDs []uint) []uint {
	if len(roleIDs) == 0 {
		return AccessibleCompanyIDs(userID)
	}

	var mappings []models.EmployeeCompanyAccess
	if err := config.DB.
		Where("employee_id = ? AND role_id IN ? AND is_active = ?", userID, roleIDs, true).
		Find(&mappings).Error; err != nil || len(mappings) == 0 {
		return nil
	}

	seen := map[uint]bool{}
	ids := make([]uint, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.CompanyID != 0 && !seen[mapping.CompanyID] {
			seen[mapping.CompanyID] = true
			ids = append(ids, mapping.CompanyID)
		}
	}
	return ids
}

func DefaultCompanyID(user models.Employees) uint {
	companyID, _ := DefaultCompanyAccess(user)
	return companyID
}

func DefaultCompanyAccess(user models.Employees) (companyID uint, roleID uint) {
	return user.CompaniesID, user.RolesID
}

func RoleNames(userID uint) []string {
	roleIDs := AccessibleRoleIDs(userID)
	if len(roleIDs) == 0 {
		return nil
	}

	var roles []models.Roles
	if err := config.DB.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
		return nil
	}

	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.RolesName)
	}
	return names
}

func CanAccessCompany(userID uint, scope models.DataScope, companyID uint) bool {
	if companyID == 0 {
		return false
	}
	for _, accessibleID := range AccessibleCompanyIDs(userID) {
		if accessibleID == companyID {
			return true
		}
	}
	return false
}

func ResolveCompanyID(user models.Employees, scope models.DataScope, requestedCompanyID uint) (uint, bool) {
	companyID := requestedCompanyID
	if companyID == 0 {
		companyID = DefaultCompanyID(user)
	}
	return companyID, CanAccessCompany(user.ID, scope, companyID)
}

func ResolveEmployeeForCompany(c *gin.Context, user models.Employees, scope models.DataScope, employeeID uint, requestedCompanyID uint) (models.Employees, bool) {
	if requestedCompanyID == 0 {
		return models.Employees{}, false
	}
	companyID, ok := ResolveCompanyID(user, scope, requestedCompanyID)
	if !ok || employeeID == 0 {
		return models.Employees{}, false
	}

	var employee models.Employees
	if err := config.DB.Scopes(
		ApplyContextScopes(c, user, scope, "employees.companies_id", "employees.id"),
	).Select("id, companies_id").First(&employee, employeeID).Error; err != nil {
		return models.Employees{}, false
	}
	if employee.CompaniesID != companyID {
		return models.Employees{}, false
	}

	return employee, true
}

func GetAuthorizedCompanyIDs(c *gin.Context, userID uint) []uint {
	if val, ok := c.Get("authorized_company_ids"); ok {
		if ids, ok := val.([]uint); ok {
			return ids
		}
	}
	return AccessibleCompanyIDs(userID)
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
			companyIDs := AccessibleCompanyIDs(user.ID)
			if len(companyIDs) == 0 {
				return db.Where("1 = 0")
			}

			subQuery := db.Session(&gorm.Session{NewDB: true}).
				Table("employees").
				Select("id").
				Where("dept_id = ? AND companies_id IN ?", user.DeptID, companyIDs)

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
			query = query.Scopes(ScopeCompanyIDs(AccessibleCompanyIDs(user.ID), companiesCol))
		}
		return query
	}
}

func ApplyContextScopes(c *gin.Context, user models.Employees, scope models.DataScope, companiesCol string, accessCol string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		companyIDs := GetAuthorizedCompanyIDs(c, user.ID)
		query := db.Scopes(scopeDataAccessWithCompanyIDs(user, scope, companyIDs, accessCol))
		if len(companyIDs) > 0 {
			query = query.Scopes(ScopeCompanyIDs(companyIDs, companiesCol))
		}
		return query
	}
}

func scopeDataAccessWithCompanyIDs(user models.Employees, scope models.DataScope, companyIDs []uint, tableCol ...string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		col := "employees_id"
		if len(tableCol) > 0 {
			col = tableCol[0]
		}
		if col == "unused" {
			return db
		}

		switch scope {
		case models.ScopeOwn:
			return db.Where(col+" = ?", user.ID)
		case models.ScopeDepartment:
			if len(companyIDs) == 0 {
				return db.Where("1 = 0")
			}
			subQuery := db.Session(&gorm.Session{NewDB: true}).
				Table("employees").
				Select("id").
				Where("dept_id = ? AND companies_id IN ?", user.DeptID, companyIDs)
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

package repository

import (
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
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

type CompanyRole struct {
	CompanyID uint
	RoleID    uint
}

// AccessibleCompanyRoles returns the role the employee acts under in each company they can reach.
func AccessibleCompanyRoles(userID uint) []CompanyRole {
	var user models.Employees
	if err := config.DB.Select("id, companies_id, roles_id").First(&user, userID).Error; err != nil {
		return nil
	}

	seen := map[uint]bool{}
	result := make([]CompanyRole, 0)

	if user.CompaniesID != 0 && user.RolesID != 0 {
		seen[user.CompaniesID] = true
		result = append(result, CompanyRole{CompanyID: user.CompaniesID, RoleID: user.RolesID})
	}

	var mappings []models.EmployeeCompanyAccess
	if err := config.DB.
		Where("employee_id = ? AND is_active = ?", userID, true).
		Find(&mappings).Error; err == nil {
		for _, m := range mappings {
			if m.CompanyID != 0 && m.RoleID != 0 && !seen[m.CompanyID] {
				seen[m.CompanyID] = true
				result = append(result, CompanyRole{CompanyID: m.CompanyID, RoleID: m.RoleID})
			}
		}
	}
	return result
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

// The *gin.Context versions of CanAccessCompany, ResolveCompanyID and ResolveEmployeeForCompany are gone.
func GetAuthorizedCompanyIDs(c *gin.Context, userID uint) []uint {
	if val, ok := c.Get("authorized_company_ids"); ok {
		if ids, ok := val.([]uint); ok {
			return ids
		}
	}
	return AccessibleCompanyIDs(userID)
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
		case models.ScopeCompanies, models.ScopeAll:
			return db
		default:
			return db.Where(employeeCol+" = ?", user.ID)
		}
	}
}

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

// AccessibleCompanyRoles returns the role the employee acts under in each
// company they can reach: their home role for their home company, and the
// mapped role for every active employee_company_access company.
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

func CanAccessCompany(c *gin.Context, userID uint, scope models.DataScope, companyID uint) bool {
	if companyID == 0 {
		return false
	}
	for _, accessibleID := range GetAuthorizedCompanyIDs(c, userID) {
		if accessibleID == companyID {
			return true
		}
	}
	return false
}

func ResolveCompanyID(c *gin.Context, user models.Employees, scope models.DataScope, requestedCompanyID uint) (uint, bool) {
	companyID := requestedCompanyID
	if companyID == 0 {
		companyID = DefaultCompanyID(user)
	}
	return companyID, CanAccessCompany(c, user.ID, scope, companyID)
}

func ResolveEmployeeForCompany(c *gin.Context, user models.Employees, scope models.DataScope, employeeID uint, requestedCompanyID uint) (models.Employees, bool) {
	if requestedCompanyID == 0 {
		return models.Employees{}, false
	}
	companyID, ok := ResolveCompanyID(c, user, scope, requestedCompanyID)
	if !ok || employeeID == 0 {
		return models.Employees{}, false
	}

	var employee models.Employees
	if err := config.DB.Scopes(
		ApplyEmployeeScopes(c, user, scope, "employees.companies_id", "employees.id"),
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

// Row-level scoping has exactly two entry points, and which one a call site
// uses is the whole statement about that table:
//
//   - ApplyCompanyScopes  — the table has no employee column. Rows belong to a
//     company, not a person (payroll cycles, holidays, master data). DataScope
//     cannot narrow these, so the function does not accept one.
//   - ApplyEmployeeScopes — the table has an employee column, so Own and
//     Department are meaningful and DataScope is honoured.
//
// Both restrict the query to `authorized_company_ids` (set by CheckPermission):
// the companies whose role actually grants the action, not merely the companies
// the user can reach. Reachability is app access, not permission to act.
//
// Picking the wrong one is a compile-time decision, not a runtime string
// comparison — that is the point of the split.

// applyCompanyFilter narrows to the authorized companies. An empty list means
// CheckPermission never ran (public or hand-rolled route), so the caller is
// responsible for its own filtering and nothing is added here.
func applyCompanyFilter(db *gorm.DB, companyIDs []uint, companiesCol string) *gorm.DB {
	if len(companyIDs) == 0 {
		return db
	}
	return db.Scopes(ScopeCompanyIDs(companyIDs, companiesCol))
}

// ApplyCompanyScopes scopes a company-owned table. There is no DataScope
// parameter because none of Own/Department/Companies would change the result.
func ApplyCompanyScopes(c *gin.Context, user models.Employees, companiesCol string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return applyCompanyFilter(db, GetAuthorizedCompanyIDs(c, user.ID), companiesCol)
	}
}

// ApplyEmployeeScopes scopes an employee-owned table: authorized companies
// first, then the role's DataScope against employeeCol.
func ApplyEmployeeScopes(c *gin.Context, user models.Employees, scope models.DataScope, companiesCol string, employeeCol string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		companyIDs := GetAuthorizedCompanyIDs(c, user.ID)
		query := db.Scopes(scopeDataAccess(user, scope, companyIDs, employeeCol))
		return applyCompanyFilter(query, companyIDs, companiesCol)
	}
}

// scopeDataAccess turns a DataScope into a predicate on employeeCol. Only
// ApplyEmployeeScopes calls it, so employeeCol always names a real column.
//
// ScopeDepartment matches on the department's *category*, not the literal
// department row: two companies' "HR" departments share one
// DepartmentCategory, so a Department-scoped role reaches its counterparts in
// every company it is authorized for. Matching on employees.dept_id instead
// would confine the user to their own single department row.
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

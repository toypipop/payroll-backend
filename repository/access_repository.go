package repository

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

// CompanyRole pairs a company the user can reach with the role they act under there.
type CompanyRole struct {
	CompanyID uint
	RoleID    uint
}

// AccessRepository answers "which companies and roles does this user act under", the lookups authentication and permission checks run before any domain query.
type AccessRepository interface {
	ResolveAccess(user models.Employees) (UserAccess, error)
	AccessibleCompanyIDs(userID uint) []uint
	AccessibleRoleIDs(userID uint) []uint
	AccessibleCompanyRoles(userID uint) []CompanyRole
	RoleNames(userID uint) []string
	FindScopeUser(userID uint) (models.Employees, error)
	ModuleAccesses(roleIDs []uint, moduleKey string) ([]models.RolesModuleAccess, error)
}

type gormAccessRepository struct {
	db *gorm.DB
}

// NewAccessRepository returns the GORM-backed implementation on the given connection.
func NewAccessRepository(db *gorm.DB) AccessRepository {
	return newGormAccessRepository(db)
}

// newGormAccessRepository lets sibling repositories reuse these lookups without going through the interface.
func newGormAccessRepository(db *gorm.DB) *gormAccessRepository {
	return &gormAccessRepository{db: db}
}

// FindScopeUser loads only the columns scoping needs, never the full row (which would drag the password hash around).
func (r *gormAccessRepository) FindScopeUser(userID uint) (models.Employees, error) {
	var user models.Employees
	// The name travels with the scoping columns because the salary audit trail snapshots who made a change,
	// and looking it up again per write would be a query to learn something the request already knew. The
	// selection stays explicit so the password hash never rides along.
	if err := r.db.Select("id, dept_id, companies_id, roles_id, first_name, last_name, is_active").First(&user, userID).Error; err != nil {
		return models.Employees{}, err
	}
	return user, nil
}

func (r *gormAccessRepository) AccessibleCompanyIDs(userID uint) []uint {
	var user models.Employees
	if err := r.db.Select("id, companies_id").First(&user, userID).Error; err != nil {
		return nil
	}

	return resolveUserAccess(user, r.activeAccess(userID)).CompanyIDs
}

// AccessibleCompanyRoles returns the role the employee acts under in each company they can reach.
func (r *gormAccessRepository) AccessibleCompanyRoles(userID uint) []CompanyRole {
	var user models.Employees
	if err := r.db.Select("id, companies_id, roles_id").First(&user, userID).Error; err != nil {
		return nil
	}

	return resolveUserAccess(user, r.activeAccess(userID)).CompanyRoles
}

func (r *gormAccessRepository) AccessibleRoleIDs(userID uint) []uint {
	var user models.Employees
	if err := r.db.Select("id, roles_id").First(&user, userID).Error; err != nil {
		return nil
	}

	return resolveUserAccess(user, r.activeAccess(userID)).RoleIDs
}

func (r *gormAccessRepository) RoleNames(userID uint) []string {
	roleIDs := r.AccessibleRoleIDs(userID)
	if len(roleIDs) == 0 {
		return nil
	}

	var roles []models.Roles
	if err := r.db.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
		return nil
	}

	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.RolesName)
	}
	return names
}

// ModuleAccesses returns the permission rows any of the roles hold on the module, for active, undeleted modules only.
func (r *gormAccessRepository) ModuleAccesses(roleIDs []uint, moduleKey string) ([]models.RolesModuleAccess, error) {
	var accesses []models.RolesModuleAccess
	// PostgreSQL does not support MySQL backtick quoting; the mixed-case GORM association alias must use double-quoted identifiers here.
	err := r.db.
		InnerJoins("AppModule", r.db.Where(`"AppModule"."deleted_at" = ?`, 0)).
		Where("roles_module_access.roles_id IN ?", roleIDs).
		Where(`"AppModule"."module_key" = ?`, moduleKey).
		Where(`"AppModule"."is_active" = ?`, true).
		Find(&accesses).Error
	return accesses, err
}

// activeAccess reads the user's extra company grants; a failed read is treated as "no extra companies", which is how the home company alone still authenticates.
func (r *gormAccessRepository) activeAccess(userID uint) []models.EmployeeCompanyAccess {
	var mappings []models.EmployeeCompanyAccess
	if err := r.db.
		Where("employee_id = ? AND is_active = ?", userID, true).
		Find(&mappings).Error; err != nil {
		return nil
	}
	return mappings
}

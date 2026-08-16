package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Auth lookup failures.
var (
	ErrAuthEmailUnknown  = apperr.Unauthorized("Invalid email")
	ErrAuthUserNotLoaded = apperr.Unauthorized("Unauthorized")
	ErrAuthModulesFailed = apperr.Internal("Failed to fetch app modules")
	ErrAuthAccessFailed  = apperr.Internal("Failed to fetch module access")
)

// AuthRepository is the persistence boundary for authentication and the module tree.
type AuthRepository interface {
	FindByEmail(email string) (models.Employees, error)
	FindEmployee(id uint) (models.Employees, error)
	ListActiveModulesOrdered() ([]models.AppModule, error)
	ListViewableModuleIDs(roleIDs []uint) (map[uint]bool, error)
	AccessibleCompanyIDs(userID uint) []uint
	AccessibleCompanyRoles(userID uint) []CompanyRole
	RoleNames(userID uint) []string
}

type gormAuthRepository struct {
	db *gorm.DB
}

// NewAuthRepository returns the GORM-backed implementation bound to the process-wide connection.
func NewAuthRepository() AuthRepository {
	return &gormAuthRepository{db: config.DB}
}

// FindByEmail loads the full row, since login needs the password hash.
func (r *gormAuthRepository) FindByEmail(email string) (models.Employees, error) {
	var employee models.Employees
	if err := r.db.Where("email = ?", email).First(&employee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Employees{}, ErrAuthEmailUnknown
		}
		return models.Employees{}, err
	}
	return employee, nil
}

func (r *gormAuthRepository) FindEmployee(id uint) (models.Employees, error) {
	var employee models.Employees
	if err := r.db.First(&employee, id).Error; err != nil {
		return models.Employees{}, ErrAuthUserNotLoaded
	}
	return employee, nil
}

// ListActiveModulesOrdered returns the active module tree in display order.
func (r *gormAuthRepository) ListActiveModulesOrdered() ([]models.AppModule, error) {
	var modules []models.AppModule
	err := r.db.
		Where("is_active = ?", true).
		// Let GORM quote the reserved column name for PostgreSQL instead of hand-writing SQL.
		Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}}).
		Find(&modules).Error
	if err != nil {
		return nil, ErrAuthModulesFailed
	}
	return modules, nil
}

// ListViewableModuleIDs returns the modules any of the given roles can view.
func (r *gormAuthRepository) ListViewableModuleIDs(roleIDs []uint) (map[uint]bool, error) {
	viewable := map[uint]bool{}
	if len(roleIDs) == 0 {
		return viewable, nil
	}

	var accesses []models.RolesModuleAccess
	if err := r.db.
		Where("roles_id IN ?", roleIDs).
		Where("can_view = ?", true).
		Find(&accesses).Error; err != nil {
		return nil, ErrAuthAccessFailed
	}
	for _, access := range accesses {
		viewable[access.ModuleID] = true
	}
	return viewable, nil
}

// The three below delegate to the package-level helpers so the service depends only on this interface and can be exercised against a fake.
func (r *gormAuthRepository) AccessibleCompanyIDs(userID uint) []uint {
	return AccessibleCompanyIDs(userID)
}

func (r *gormAuthRepository) AccessibleCompanyRoles(userID uint) []CompanyRole {
	return AccessibleCompanyRoles(userID)
}

func (r *gormAuthRepository) RoleNames(userID uint) []string {
	return RoleNames(userID)
}

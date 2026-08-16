package repository

import (
	"errors"
	"payroll-backend/config"
	"payroll-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ErrNoAuthContext means the request reached a handler without a usable authenticated user — AuthMiddleware should have made this impossible.
var ErrNoAuthContext = errors.New("no authenticated user in request context")

// AccessScope is the request's resolved authorization context, lifted out of gin so lower layers never see a request.
type AccessScope struct {
	// User carries only id, dept_id and companies_id — enough for scoping, never the full row (which would drag the password hash around).
	User models.Employees
	// DataScope is the widest scope the caller's roles grant for this action.
	DataScope models.DataScope
	// AuthorizedCompanyIDs are the companies whose role actually grants the action being performed — not merely the companies the user can reach.
	AuthorizedCompanyIDs []uint
}

// NewAccessScope resolves the caller's identity and authorization once, so handlers stop repeating the "load currentUser, then read data_scope" preamble.
func NewAccessScope(c *gin.Context) (AccessScope, error) {
	userID, dataScope := GetContextAuth(c)
	if userID == 0 {
		return AccessScope{}, ErrNoAuthContext
	}

	var user models.Employees
	if err := config.DB.Select("id, dept_id, companies_id").First(&user, userID).Error; err != nil {
		return AccessScope{}, ErrNoAuthContext
	}

	return AccessScope{
		User:                 user,
		DataScope:            dataScope,
		AuthorizedCompanyIDs: GetAuthorizedCompanyIDs(c, userID),
	}, nil
}

// ApplyCompany is the AccessScope equivalent of ApplyCompanyScopes: for tables whose rows belong to a company rather than a person.
func (s AccessScope) ApplyCompany(companiesCol string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return applyCompanyFilter(db, s.AuthorizedCompanyIDs, companiesCol)
	}
}

// ApplyEmployee is the AccessScope equivalent of ApplyEmployeeScopes: for tables with a real employee column, where Own and Department are meaningful.
func (s AccessScope) ApplyEmployee(companiesCol string, employeeCol string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		query := db.Scopes(scopeDataAccess(s.User, s.DataScope, s.AuthorizedCompanyIDs, employeeCol))
		return applyCompanyFilter(query, s.AuthorizedCompanyIDs, companiesCol)
	}
}

// CanAccessCompany reports whether the caller may act on the given company.
func (s AccessScope) CanAccessCompany(companyID uint) bool {
	if companyID == 0 {
		return false
	}
	for _, id := range s.AuthorizedCompanyIDs {
		if id == companyID {
			return true
		}
	}
	return false
}

// ResolveCompanyID falls back to the caller's home company when the request omits one, then checks the result is actually authorized.
func (s AccessScope) ResolveCompanyID(requestedCompanyID uint) (uint, bool) {
	companyID := requestedCompanyID
	if companyID == 0 {
		companyID = s.User.CompaniesID
	}
	return companyID, s.CanAccessCompany(companyID)
}

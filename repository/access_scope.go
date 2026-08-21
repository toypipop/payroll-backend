package repository

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

// AccessScope is the request's resolved authorization context, lifted out of gin so lower layers never see a request.
type AccessScope struct {
	// User carries only id, dept_id and companies_id — enough for scoping, never the full row (which would drag the password hash around).
	User models.Employees
	// DataScope is the widest scope the caller's roles grant for this action.
	DataScope models.DataScope
	// AuthorizedCompanyIDs are the companies whose role actually grants the action being performed — not merely the companies the user can reach.
	AuthorizedCompanyIDs []uint
}

// NewAccessScope takes values the caller has already resolved, so building a scope needs neither a database handle nor a request.
func NewAccessScope(user models.Employees, dataScope models.DataScope, authorizedCompanyIDs []uint) AccessScope {
	return AccessScope{
		User:                 user,
		DataScope:            dataScope,
		AuthorizedCompanyIDs: authorizedCompanyIDs,
	}
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

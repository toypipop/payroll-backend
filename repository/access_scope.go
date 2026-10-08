package repository

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

// AccessScope is the request's resolved authorization context, lifted out of gin so lower layers never see a request.
type AccessScope struct {
	// User carries only id, dept_id, companies_id and the name — enough for scoping and for naming who made a
	// change, never the full row (which would drag the password hash around).
	User models.Employees
	// DataScope is the fallback for callers without per-company permission grants.
	DataScope models.DataScope
	// CompanyDataScopes keeps a broader grant in one company from widening another.
	// Nil is reserved for scopes constructed without a module permission gate.
	CompanyDataScopes map[uint]models.DataScope
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
		if s.CompanyDataScopes != nil {
			// Build grouped OR branches, each with both its company and employee
			// predicate. Never let a branch escape existing ID or business filters.
			branches := db.Session(&gorm.Session{NewDB: true}).Where("1 = 0")
			for _, dataScope := range []models.DataScope{models.ScopeCompanies, models.ScopeDepartment, models.ScopeOwn} {
				var companyIDs []uint
				for _, companyID := range s.AuthorizedCompanyIDs {
					if s.CompanyDataScopes[companyID] == dataScope {
						companyIDs = append(companyIDs, companyID)
					}
				}
				if len(companyIDs) == 0 {
					continue
				}
				branch := db.Session(&gorm.Session{NewDB: true}).
					Where(companiesCol+" IN ?", companyIDs).
					Scopes(scopeDataAccess(s.User, dataScope, companyIDs, employeeCol))
				branches = branches.Or(branch)
			}
			return db.Where(branches)
		}
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

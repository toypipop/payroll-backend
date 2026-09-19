package repository

import "payroll-backend/models"

// UserAccess is resolved once per request and reused by permission middleware.
// It must not be cached across requests: grants can change while a cookie is valid.
type UserAccess struct {
	CompanyIDs   []uint
	RoleIDs      []uint
	CompanyRoles []CompanyRole
}

// ResolveAccess reuses the authenticated employee and reads extra grants once.
// A failed lookup denies the request rather than treating missing data as no grants.
func (r *gormAccessRepository) ResolveAccess(user models.Employees) (UserAccess, error) {
	var mappings []models.EmployeeCompanyAccess
	if err := r.db.Select("company_id, role_id").
		Where("employee_id = ? AND is_active = ?", user.ID, true).
		Order("id ASC").Find(&mappings).Error; err != nil {
		return UserAccess{}, err
	}
	return resolveUserAccess(user, mappings), nil
}

func resolveUserAccess(user models.Employees, mappings []models.EmployeeCompanyAccess) UserAccess {
	access := UserAccess{CompanyIDs: make([]uint, 0), RoleIDs: make([]uint, 0), CompanyRoles: make([]CompanyRole, 0)}
	companies, roles, pairedCompanies := map[uint]bool{}, map[uint]bool{}, map[uint]bool{}
	add := func(companyID, roleID uint) {
		if companyID != 0 && !companies[companyID] {
			companies[companyID] = true
			access.CompanyIDs = append(access.CompanyIDs, companyID)
		}
		if roleID != 0 && !roles[roleID] {
			roles[roleID] = true
			access.RoleIDs = append(access.RoleIDs, roleID)
		}
		if companyID != 0 && roleID != 0 && !pairedCompanies[companyID] {
			pairedCompanies[companyID] = true
			access.CompanyRoles = append(access.CompanyRoles, CompanyRole{CompanyID: companyID, RoleID: roleID})
		}
	}
	// Home-company role takes precedence over any extra mapping for that company.
	add(user.CompaniesID, user.RolesID)
	for _, mapping := range mappings {
		add(mapping.CompanyID, mapping.RoleID)
	}
	return access
}

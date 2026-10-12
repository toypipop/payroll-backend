// Package testfixture supplies explicit authorization dependencies to domain tests.
package testfixture

import (
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"
)

type access struct {
	repository.AccessRepository
	companyID uint
}

func (a access) ResolveAccess(models.Employees) (repository.UserAccess, error) {
	return repository.UserAccess{CompanyRoles: []repository.CompanyRole{{CompanyID: a.companyID, RoleID: 99}}}, nil
}
func (a access) ModuleAccesses([]uint, string) ([]models.RolesModuleAccess, error) {
	return []models.RolesModuleAccess{{CompaniesID: a.companyID, RolesID: 99, CanEdit: true, DataScope: models.ScopeCompanies}}, nil
}
func RoleAssignments(companyID uint) *service.RoleAssignmentService {
	return service.NewRoleAssignmentService(access{companyID: companyID})
}

type locations struct {
	repository.WorkLocationRepository
	companyID uint
}

func (l locations) FindScoped(_ repository.AccessScope, id uint) (models.WorkLocation, error) {
	return models.WorkLocation{ID: id, CompaniesID: l.companyID}, nil
}
func WorkLocations(companyID uint) repository.WorkLocationRepository {
	return locations{companyID: companyID}
}

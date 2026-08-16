package service

import (
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"

	"golang.org/x/crypto/bcrypt"
)

// Auth failures.
var (
	ErrAuthPasswordWrong = apperr.Unauthorized("Invalid password")
	ErrAuthNoRoleAccess  = apperr.Forbidden("No role access configured")
)

// Re-exported so handlers import a single package.
var (
	ErrAuthEmailUnknown  = repository.ErrAuthEmailUnknown
	ErrAuthUserNotLoaded = repository.ErrAuthUserNotLoaded
)

// TokenSubject is what a freshly issued token says about who the holder is.
type TokenSubject struct {
	UserID      uint
	CompaniesID uint
	RolesID     uint
}

// AuthService owns credential checking and the shape of the identity a signed-in
// caller sees. Signing the token and writing the cookie stay in the handler, because
// those are HTTP concerns rather than rules.
type AuthService struct {
	repo repository.AuthRepository
}

// NewAuthService wires the service to a repository implementation.
func NewAuthService(repo repository.AuthRepository) *AuthService {
	return &AuthService{repo: repo}
}

// Authenticate checks an email and password pair. An unknown email and a wrong
// password answer differently, as this API has always done.
func (s *AuthService) Authenticate(email string, password string) (models.Employees, error) {
	employee, err := s.repo.FindByEmail(email)
	if err != nil {
		return models.Employees{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(employee.Password), []byte(password)); err != nil {
		return models.Employees{}, ErrAuthPasswordWrong
	}
	return employee, nil
}

// Subject resolves what to put in a token for an authenticated employee.
func (s *AuthService) Subject(employee models.Employees) TokenSubject {
	companyID, roleID := repository.DefaultCompanyAccess(employee)
	return TokenSubject{UserID: employee.ID, CompaniesID: companyID, RolesID: roleID}
}

// Identity is the profile a signed-in caller sees, for both login and /auth/me.
//
// The stored fields are deliberately few: id, name, roles and the companies the
// caller can reach. Nothing here is authorisation — permission is always resolved
// server-side per request.
func (s *AuthService) Identity(employee models.Employees) models.LoginResponse {
	companyID := repository.DefaultCompanyID(employee)
	companyIDs := s.repo.AccessibleCompanyIDs(employee.ID)
	// A caller whose only company is their home company still needs it listed.
	if len(companyIDs) == 0 && companyID != 0 {
		companyIDs = []uint{companyID}
	}

	return models.LoginResponse{
		ID:                   employee.ID,
		FullName:             employee.FirstName + " " + employee.LastName,
		Roles:                s.repo.RoleNames(employee.ID),
		CompaniesID:          companyID,
		AccessibleCompanyIDs: companyIDs,
	}
}

// IdentityByID loads an employee and returns their profile.
func (s *AuthService) IdentityByID(userID uint) (models.LoginResponse, error) {
	employee, err := s.repo.FindEmployee(userID)
	if err != nil {
		return models.LoginResponse{}, err
	}
	return s.Identity(employee), nil
}

// AllModules returns every active module, unfiltered. The Permission Settings screen
// needs the whole tree regardless of the admin's own access.
func (s *AuthService) AllModules() ([]models.AppModule, error) {
	return s.repo.ListActiveModulesOrdered()
}

// MyModules returns the active modules the caller can actually view: a leaf survives
// if any of their roles, across every company they can reach, grants CanView on it,
// and a category survives only if one of its children did. This is what the sidebar,
// the route table and the "Set it up" links read.
func (s *AuthService) MyModules(userID uint) ([]models.AppModule, error) {
	modules, err := s.repo.ListActiveModulesOrdered()
	if err != nil {
		return nil, err
	}

	companyRoles := s.repo.AccessibleCompanyRoles(userID)
	roleIDs := make([]uint, 0, len(companyRoles))
	for _, companyRole := range companyRoles {
		roleIDs = append(roleIDs, companyRole.RoleID)
	}

	viewable, err := s.repo.ListViewableModuleIDs(roleIDs)
	if err != nil {
		return nil, err
	}

	viewableParents := map[uint]bool{}
	for _, module := range modules {
		if !module.IsCategory && viewable[module.ID] && module.ParentID != nil {
			viewableParents[*module.ParentID] = true
		}
	}

	filtered := make([]models.AppModule, 0, len(modules))
	for _, module := range modules {
		if module.IsCategory {
			if viewableParents[module.ID] {
				filtered = append(filtered, module)
			}
			continue
		}
		if viewable[module.ID] {
			filtered = append(filtered, module)
		}
	}
	return filtered, nil
}

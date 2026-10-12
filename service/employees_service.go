package service

import (
	"errors"
	"math"
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost matches what the login path expects to verify against.
const bcryptCost = 12

const (
	minimumPasswordLength = 6
	maximumPasswordLength = 30
)

// Business-rule failures; the kind decides the status, so nothing here names an HTTP code.
var (
	ErrCompanyAccessDenied = apperr.Forbidden("Access denied for this company")
	ErrRoleCompanyMismatch = apperr.Validation("Role does not belong to selected company")
	ErrPasswordTooShort    = apperr.Validation("Password must be at least 6 characters")
	ErrPasswordTooLong     = apperr.Validation("Password must be at most 30 characters")
	ErrPasswordHash        = errors.New("could not secure the password")
)

// ErrEmployeeNotFound is re-exported so handlers import a single package.
var ErrEmployeeNotFound = repository.ErrEmployeeNotFound

// EmployeesService owns the employee business rules: which company a record may belong to, that a role must match that company, and how credentials are stored.
type EmployeesService struct {
	repo        repository.EmployeesRepository
	sessions    *SessionService
	assignments *RoleAssignmentService
}

// NewEmployeesService wires the service to a repository implementation.
func NewEmployeesService(repo repository.EmployeesRepository, assignments *RoleAssignmentService, sessions ...*SessionService) *EmployeesService {
	var sessionService *SessionService
	if len(sessions) > 0 {
		sessionService = sessions[0]
	}
	return &EmployeesService{repo: repo, sessions: sessionService, assignments: assignments}
}

func (s *EmployeesService) List(scope repository.AccessScope, q repository.PageQuery) (repository.Page[models.Employees], error) {
	return s.repo.ListScoped(scope, q)
}

// assertRoleBelongsTo enforces the rule that a role is company-owned: an employee can never be given a role defined under a different company.
func (s *EmployeesService) assertRoleBelongsTo(roleID uint, companyID uint) error {
	role, err := s.repo.FindRole(roleID)
	if err != nil {
		return ErrRoleCompanyMismatch
	}
	if role.CompaniesID != companyID {
		return ErrRoleCompanyMismatch
	}
	return nil
}

// Create registers a new employee under a company the caller is authorized for.
func (s *EmployeesService) Create(scope repository.AccessScope, input models.Employees) (models.Employees, error) {
	companyID, ok := scope.ResolveCompanyID(input.CompaniesID)
	if !ok {
		return models.Employees{}, ErrCompanyAccessDenied
	}
	input.CompaniesID = companyID
	if err := normalizeEmployeeProfile(&input); err != nil {
		return models.Employees{}, err
	}

	if err := s.assertRoleBelongsTo(input.RolesID, companyID); err != nil {
		return models.Employees{}, err
	}

	if err := validatePassword(input.Password); err != nil {
		return models.Employees{}, err
	}
	if err := s.assignments.Require(scope, companyID); err != nil {
		return models.Employees{}, err
	}
	if err := s.repo.ValidateCompanyReferences(input); err != nil {
		return models.Employees{}, err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcryptCost)
	if err != nil {
		return models.Employees{}, ErrPasswordHash
	}
	input.Password = string(hashed)

	if err := s.repo.WithinTx(func(repo repository.EmployeesRepository) error {
		if err := repo.Create(&input); err != nil {
			return err
		}
		return appendEmployeeAudit(repo, scope, models.AuditEventEmployeeCreated, "Employee created", input, nil, employeeAuditData(input))
	}); err != nil {
		return models.Employees{}, err
	}
	return input, nil
}

// Update applies a partial patch.
func (s *EmployeesService) Update(scope repository.AccessScope, id uint, patch map[string]any) (models.Employees, error) {
	employee, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return models.Employees{}, err
	}

	patch = pickEmployeeFields(patch)
	if err := validateEmployeeProfilePatch(patch); err != nil {
		return models.Employees{}, err
	}

	companyID, ok := scope.ResolveCompanyID(employee.CompaniesID)
	if !ok {
		return models.Employees{}, ErrCompanyAccessDenied
	}

	nextRoleID := employee.RolesID
	if value, exists := patch["roles_id"]; exists {
		id, err := employeePatchID(value, false)
		if err != nil || id == 0 {
			return models.Employees{}, apperr.Validation("Invalid role ID")
		}
		nextRoleID = id
		patch["roles_id"] = id
	}
	if nextRoleID != employee.RolesID {
		if err := s.assignments.Require(scope, companyID); err != nil {
			return models.Employees{}, err
		}
	}
	if err := s.assertRoleBelongsTo(nextRoleID, companyID); err != nil {
		return models.Employees{}, err
	}

	passwordChanged := false
	if password, ok := patch["password"].(string); ok && password != "" {
		if err := validatePassword(password); err != nil {
			return models.Employees{}, err
		}
		hashed, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
		if hashErr != nil {
			return models.Employees{}, ErrPasswordHash
		}
		patch["password"] = string(hashed)
		passwordChanged = true
	} else {
		delete(patch, "password")
	}

	if raw, exists := patch["bank_id"]; exists {
		id, err := employeePatchID(raw, true)
		if err != nil {
			return models.Employees{}, err
		}
		if id == 0 {
			patch["bank_id"] = repository.ClearField
		} else {
			patch["bank_id"] = id
		}
	}
	if err := s.validateReferencePatch(scope, employee, patch); err != nil {
		return models.Employees{}, err
	}

	before := employeeAuditData(employee)
	if err := s.repo.WithinTx(func(repo repository.EmployeesRepository) error {
		// A password reset or account deactivation must invalidate every device.
		// Revocation happens first: if the employee write later fails, signing out is
		// still the safe outcome rather than leaving a possibly compromised session alive.
		active, statusChanged := patch["is_active"].(bool)
		if s.sessions != nil && (passwordChanged || (statusChanged && !active)) {
			if err := s.sessions.RevokeAll(employee.ID, time.Now()); err != nil {
				return err
			}
		}
		if err := repo.UpdateFields(&employee, patch); err != nil {
			return err
		}
		return appendEmployeeAudit(repo, scope, models.AuditEventEmployeeUpdated, "Employee updated", employee, before, employeeAuditPatch(patch))
	}); err != nil {
		return models.Employees{}, err
	}
	return s.repo.FindDetail(id)
}

// validatePassword mirrors the employee form's 6–30-character range and
// keeps direct API callers from bypassing the UI validation.
func validatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < minimumPasswordLength {
		return ErrPasswordTooShort
	}
	if length > maximumPasswordLength {
		return ErrPasswordTooLong
	}
	return nil
}

// employeeUpdatableFields is every column a client may patch on an employee.
// It is an allowlist rather than a list of forbidden keys because the patch goes straight into an UPDATE: anything not named here — companies_id, id, created_at, deleted_at, or a column added later — is dropped instead of silently written.
var employeeUpdatableFields = map[string]bool{
	"employees_code":     true,
	"email":              true,
	"first_name":         true,
	"last_name":          true,
	"password":           true,
	"dept_id":            true,
	"pos_id":             true,
	"roles_id":           true,
	"employment_type_id": true,
	"is_active":          true,
	"bank_id":            true,
	"account_name":       true,
	"account_no":         true,
}

// pickEmployeeFields keeps only the patch keys an employee update is allowed to write.
func pickEmployeeFields(patch map[string]any) map[string]any {
	allowed := make(map[string]any, len(patch))
	for key, value := range patch {
		if employeeUpdatableFields[key] {
			allowed[key] = value
		}
	}
	return allowed
}

// validateReferencePatch validates the resulting relationships before writing.
func (s *EmployeesService) validateReferencePatch(scope repository.AccessScope, employee models.Employees, patch map[string]any) error {
	references := employee
	changed := false
	if bank, exists := patch["bank_id"]; exists {
		changed = true
		if id, ok := bank.(uint); ok {
			references.BankID = &id
		} else {
			references.BankID = nil
		}
	}
	for _, field := range []string{"dept_id", "pos_id", "employment_type_id"} {
		value, exists := patch[field]
		if !exists {
			continue
		}
		changed = true
		id, err := employeePatchID(value, field == "employment_type_id")
		if err != nil {
			return err
		}
		if id == 0 && field != "employment_type_id" {
			return apperr.Validation(field + " is required")
		}
		switch field {
		case "dept_id":
			references.DeptID = id
		case "pos_id":
			references.PosID = id
		case "employment_type_id":
			if id == 0 {
				references.EmploymentTypeID = nil
				patch[field] = repository.ClearField
				continue
			}
			references.EmploymentTypeID = &id
		}
		patch[field] = id
	}
	if !changed {
		return nil
	}
	// Department defines row visibility; a restricted editor cannot expand their
	// scope by moving themselves to another department.
	if references.DeptID != employee.DeptID {
		dataScope := scope.DataScope
		if scope.CompanyDataScopes != nil {
			dataScope = scope.CompanyDataScopes[employee.CompaniesID]
		}
		if dataScope != models.ScopeCompanies {
			if err := s.assignments.Require(scope, employee.CompaniesID); err != nil {
				return err
			}
		}
	}
	return s.repo.ValidateCompanyReferences(references)
}

func employeePatchID(value any, nullable bool) (uint, error) {
	if value == nil && nullable {
		return 0, nil
	}
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number > 9007199254740991 || math.Trunc(number) != number {
		return 0, apperr.Validation("Invalid employee reference ID")
	}
	return uint(number), nil
}

// Delete removes an employee the caller is scoped to see.
func (s *EmployeesService) Delete(scope repository.AccessScope, id uint) error {
	employee, err := s.repo.FindScoped(scope, id)
	if err != nil {
		return err
	}
	return s.repo.WithinTx(func(repo repository.EmployeesRepository) error {
		if err := repo.Delete(&employee); err != nil {
			return err
		}
		return appendEmployeeAudit(repo, scope, models.AuditEventEmployeeDeleted, "Employee deleted", employee, employeeAuditData(employee), nil)
	})
}

func appendEmployeeAudit(repo repository.EmployeesRepository, scope repository.AccessScope, event models.AuditEventType, summary string, employee models.Employees, before, after map[string]any) error {
	companyID, employeeID := employee.CompaniesID, employee.ID
	audit, err := BuildAuditLog(scope, models.AuditLogInput{CompaniesID: &companyID, EntityID: &employeeID, EventType: event, EntityType: "employee", Summary: summary, BeforeData: before, AfterData: after})
	if err != nil {
		return err
	}
	return repo.AppendAuditLog(&audit)
}

func employeeAuditData(employee models.Employees) map[string]any {
	return map[string]any{"employees_code": employee.EmployeesCode, "first_name": employee.FirstName, "last_name": employee.LastName, "roles_id": employee.RolesID, "is_active": employee.IsActive}
}

func employeeAuditPatch(patch map[string]any) map[string]any {
	safe := map[string]any{}
	for _, key := range []string{"employees_code", "first_name", "last_name", "roles_id", "is_active", "dept_id", "pos_id", "employment_type_id"} {
		if value, ok := patch[key]; ok {
			safe[key] = value
		}
	}
	return safe
}

package service

import (
	"errors"
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
)

// fakePermissionRepo stands in for the matrix's persistence boundary and records what Replace
// actually asked it to write.
type fakePermissionRepo struct {
	capabilities repository.ModuleCapabilities
	capErr       error

	askedFor []uint
	written  []models.RolesModuleAccess
	writes   int
}

func (f *fakePermissionRepo) ListModulesOrdered() ([]models.AppModule, error) { return nil, nil }

func (f *fakePermissionRepo) ListRoleAccessScoped(repository.AccessScope, uint) ([]models.RolesModuleAccess, error) {
	return nil, nil
}

func (f *fakePermissionRepo) CapabilitiesFor(moduleIDs []uint) (repository.ModuleCapabilities, error) {
	f.askedFor = append([]uint(nil), moduleIDs...)
	if f.capErr != nil {
		return repository.ModuleCapabilities{}, f.capErr
	}
	return f.capabilities, nil
}

func (f *fakePermissionRepo) ReplaceRoleAccess(_ repository.AccessScope, _ uint, permissions []models.RolesModuleAccess, _ *models.AuditLog) error {
	f.writes++
	f.written = append([]models.RolesModuleAccess(nil), permissions...)
	return nil
}

// fakeRoleRepo resolves the role Replace writes against; only FindScoped is exercised.
type fakeRoleRepo struct {
	role models.Roles
	err  error
}

func (f *fakeRoleRepo) ListScoped(repository.AccessScope, repository.PageQuery) (repository.Page[models.Roles], error) {
	return repository.Page[models.Roles]{}, nil
}
func (f *fakeRoleRepo) ListMinimalScoped(repository.AccessScope) ([]models.Roles, error) {
	return nil, nil
}
func (f *fakeRoleRepo) FindScoped(repository.AccessScope, uint) (models.Roles, error) {
	return f.role, f.err
}
func (f *fakeRoleRepo) Create(*models.Roles) error                              { return nil }
func (f *fakeRoleRepo) UpdateName(*models.Roles, string) error                  { return nil }
func (f *fakeRoleRepo) Delete(*models.Roles) error                              { return nil }
func (f *fakeRoleRepo) WithinTx(fn func(repository.RoleRepository) error) error { return fn(f) }
func (f *fakeRoleRepo) AppendAuditLog(*models.AuditLog) error                   { return nil }

func newPermissionService(repo *fakePermissionRepo) *PermissionService {
	return NewPermissionService(repo, &fakeRoleRepo{role: models.Roles{CompaniesID: 1}})
}

// The matrix posts every row of the module tree back, including the headings that group the pages.
// A heading is not a page: it has no capability, and treating it as an unknown module used to reject
// the entire save, which left the permission screen unable to write anything at all.
func TestReplaceSkipsCategoryModulesInsteadOfRejectingThePayload(t *testing.T) {
	repo := &fakePermissionRepo{
		capabilities: repository.ModuleCapabilities{
			ByModule: map[uint]models.ModuleCapability{
				9: {ScopeLevel: models.ScopeLevelEmployee, AllowedActions: []string{models.ActionView, models.ActionEdit}},
			},
			Categories: map[uint]bool{2: true},
		},
	}

	err := newPermissionService(repo).Replace(repository.AccessScope{}, 1, []PermissionEntry{
		{ModuleID: 2, CanView: true, DataScope: models.ScopeCompanies},
		{ModuleID: 9, CanView: true, CanEdit: true, DataScope: models.ScopeCompanies},
	})
	if err != nil {
		t.Fatalf("Replace returned %v, want nil", err)
	}
	if repo.writes != 1 {
		t.Fatalf("wrote %d times, want 1", repo.writes)
	}
	if len(repo.written) != 1 {
		t.Fatalf("wrote %d permission rows, want 1 (the category carries none)", len(repo.written))
	}
	if repo.written[0].ModuleID != 9 {
		t.Fatalf("wrote module %d, want 9", repo.written[0].ModuleID)
	}
	if !repo.written[0].CanEdit {
		t.Fatal("CanEdit was dropped for a module whose capability allows EDIT")
	}
}

// An id the repository cannot resolve at all is still a rejection, and it must be rejected before
// anything is written.
func TestReplaceRejectsAnUnknownModuleWithoutWriting(t *testing.T) {
	repo := &fakePermissionRepo{capErr: repository.ErrUnknownModule}

	err := newPermissionService(repo).Replace(repository.AccessScope{}, 1, []PermissionEntry{
		{ModuleID: 404, CanView: true, DataScope: models.ScopeCompanies},
	})
	if !errors.Is(err, repository.ErrUnknownModule) {
		t.Fatalf("Replace returned %v, want ErrUnknownModule", err)
	}
	if repo.writes != 0 {
		t.Fatalf("wrote %d times on a rejected payload, want 0", repo.writes)
	}
}

// An action the module's capability does not declare is coerced away rather than stored, so the
// database never holds a claim CheckPermission would ignore.
func TestReplaceDropsAnActionTheCapabilityDoesNotDeclare(t *testing.T) {
	repo := &fakePermissionRepo{
		capabilities: repository.ModuleCapabilities{
			ByModule: map[uint]models.ModuleCapability{
				9: {ScopeLevel: models.ScopeLevelCompany, AllowedActions: []string{models.ActionView}},
			},
			Categories: map[uint]bool{},
		},
	}

	if err := newPermissionService(repo).Replace(repository.AccessScope{}, 1, []PermissionEntry{
		{ModuleID: 9, CanView: true, CanEdit: true, DataScope: models.ScopeOwn},
	}); err != nil {
		t.Fatalf("Replace returned %v, want nil", err)
	}
	row := repo.written[0]
	if row.CanEdit {
		t.Fatal("CanEdit was stored for a module whose capability does not allow EDIT")
	}
	if row.DataScope != models.ScopeCompanies {
		t.Fatalf("DataScope is %q, want Companies — a company-level module cannot honour Own", row.DataScope)
	}
}

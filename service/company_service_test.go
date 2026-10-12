package service

import (
	"encoding/json"
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
)

type lifecycleCompanyRepo struct {
	repository.CompanyRepository
	created   models.Companies
	bootstrap repository.TenantBootstrap
}

func (r *lifecycleCompanyRepo) CreateTenant(company *models.Companies, boot repository.TenantBootstrap, _ *models.AuditLog) error {
	r.created = *company
	r.bootstrap = boot
	return nil
}

func TestCompanyCreateAlwaysStartsActive(t *testing.T) {
	repo := &lifecycleCompanyRepo{}
	var input models.Companies
	if err := json.Unmarshal([]byte(`{"name":" Example ","tax_id":"1234567890123","address":"Example address","phone":"0812345678","is_active":false}`), &input); err != nil {
		t.Fatal(err)
	}
	company, err := NewCompanyService(repo).Create(repository.NewAccessScope(models.Employees{ID: 1}, models.ScopeCompanies, nil), input)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(company)
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatal(err)
	}
	if response["is_active"] != true || repo.created.Name != "Example" {
		t.Fatalf("new company must be active with a trimmed name: %s", encoded)
	}
}

func TestCompanyBootstrapKeepsServerOwnedIdentityAndGrants(t *testing.T) {
	r := &lifecycleCompanyRepo{}
	svc := NewCompanyService(r)
	scope := repository.NewAccessScope(models.Employees{ID: 9}, models.ScopeCompanies, []uint{1})
	_, err := svc.Create(scope, models.Companies{ID: 99, Name: "New", TaxID: "1234567890123", Address: "Example address", Phone: "0812345678", PayrollConfig: &models.PayrollConfig{ID: 77}, AttendanceConfig: &models.AttendanceConfig{ID: 88}, EmployeeAccess: []models.EmployeeCompanyAccess{{EmployeeID: 100, RoleID: 90}}})
	if err != nil {
		t.Fatal(err)
	}
	if r.created.ID != 0 || r.created.PayrollConfig != nil || r.created.AttendanceConfig != nil || len(r.created.EmployeeAccess) != 0 {
		t.Fatal("client associations reached persistence")
	}
	if r.bootstrap.CreatorID != 9 || r.bootstrap.RoleName != "ADMIN" || !r.bootstrap.Grant.CanEdit || !r.bootstrap.Grant.CanCreate || !r.bootstrap.Grant.CanView || !r.bootstrap.Grant.CanDelete {
		t.Fatal("server bootstrap lost")
	}
}

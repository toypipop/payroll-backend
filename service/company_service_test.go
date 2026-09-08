package service

import (
	"encoding/json"
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
)

type lifecycleCompanyRepo struct {
	repository.CompanyRepository
	created models.Companies
}

func (r *lifecycleCompanyRepo) CreateTenant(company *models.Companies, _ repository.TenantBootstrap, _ *models.AuditLog) error {
	r.created = *company
	return nil
}

func TestCompanyCreateAlwaysStartsActive(t *testing.T) {
	repo := &lifecycleCompanyRepo{}
	var input models.Companies
	if err := json.Unmarshal([]byte(`{"name":" Example ","is_active":false}`), &input); err != nil {
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

package service

import (
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
)

type auditLogRepoFake struct{ entries []models.AuditLog }

func (r *auditLogRepoFake) Append(entry *models.AuditLog) error {
	r.entries = append(r.entries, *entry)
	return nil
}
func (r *auditLogRepoFake) ListScoped(repository.AccessScope, *uint) ([]models.AuditLog, error) {
	return r.entries, nil
}

func TestAuditLogServiceSnapshotsActorFromScope(t *testing.T) {
	repo := &auditLogRepoFake{}
	svc := NewAuditLogService(repo)
	companyID, entityID := uint(2), uint(7)
	err := svc.Record(repository.NewAccessScope(models.Employees{ID: 9, FirstName: "Ada", LastName: "Admin"}, models.ScopeCompanies, []uint{2}), models.AuditLogInput{
		CompaniesID: &companyID, EntityID: &entityID, EventType: models.AuditEventEmployeeUpdated,
		EntityType: "employee", Summary: "Updated employee", AfterData: map[string]any{"first_name": "Grace"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.entries) != 1 || repo.entries[0].ActorNameSnapshot != "Ada Admin" {
		t.Fatalf("entries = %#v", repo.entries)
	}
}

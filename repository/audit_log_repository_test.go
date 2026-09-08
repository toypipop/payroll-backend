package repository

import (
	"strings"
	"testing"

	"payroll-backend/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func auditQuerySQL(t *testing.T, scope AccessScope) string {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=invalid"}), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true, NamingStrategy: schema.NamingStrategy{SingularTable: true}, Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	query := (&gormAuditLogRepository{db: db}).scoped(scope).Order("occurred_at DESC, id DESC").Find(&[]models.AuditLog{})
	return query.Statement.SQL.String()
}

func TestAuditLogAlwaysFiltersAuthorizedCompanies(t *testing.T) {
	sql := auditQuerySQL(t, NewAccessScope(models.Employees{ID: 1}, models.ScopeCompanies, []uint{1}))
	if !strings.Contains(sql, "companies_id IN") {
		t.Fatalf("Audit logs must always filter authorized companies: %s", sql)
	}
}

func TestAuditLogScopeCompaniesFiltersAuthorizedCompanies(t *testing.T) {
	sql := auditQuerySQL(t, NewAccessScope(models.Employees{ID: 1}, models.ScopeCompanies, []uint{2, 3}))
	if !strings.Contains(sql, "companies_id IN") {
		t.Fatalf("Companies scope must filter companies: %s", sql)
	}
}

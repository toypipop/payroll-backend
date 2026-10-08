package repository

import (
	"database/sql/driver"
	"errors"
	"testing"

	"payroll-backend/models"
)

func TestPayrollConfigUpsertCannotChangeCompanyThroughAssociation(t *testing.T) {
	for _, failAudit := range []bool{false, true} {
		name := "commit"
		if failAudit {
			name = "rollback on audit failure"
		}
		t.Run(name, func(t *testing.T) {
			auditFailure := errors.New("audit write failed")
			auditStep := companySQLStep{contains: []string{`INSERT INTO "audit_log"`}, columns: []string{"id"}, values: []driver.Value{int64(1)}}
			if failAudit {
				auditStep.err = auditFailure
			}
			base, boundary := lifecycleRepo(t,
				companySQLStep{
					contains: []string{`INSERT INTO "payroll_config"`, `ON CONFLICT ("companies_id") DO UPDATE SET`},
					columns:  []string{"id"}, values: []driver.Value{int64(10)},
					checkArgs: func(args []driver.NamedValue) {
						// First column is companies_id. No association SQL may precede this write.
						if len(args) == 0 || args[0].Value != int64(1) {
							t.Fatalf("write targeted wrong company: %v", args)
						}
					},
				}, auditStep,
			)
			config := models.PayrollConfig{CompaniesID: 1, Companies: models.Companies{ID: 2}, SsoRate: 3, SsoMaxAmount: 650, SsoMinSalary: 1550, TaxCalculation: "Progressive"}
			err := NewPayrollConfigRepository(base.(*gormCompanyRepository).db).UpsertForCompany(&config, &models.AuditLog{})
			if config.CompaniesID != 1 {
				t.Fatalf("company changed to %d", config.CompaniesID)
			}
			if failAudit {
				if !errors.Is(err, auditFailure) || !boundary.rolledBack || boundary.committed {
					t.Fatalf("missing rollback: %v", err)
				}
			} else if err != nil || !boundary.committed || boundary.rolledBack {
				t.Fatalf("missing atomic commit: %v", err)
			}
		})
	}
}

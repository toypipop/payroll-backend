package repository

import (
	"database/sql/driver"
	"errors"
	"payroll-backend/migrations"
	"testing"
)

func copiedColumn(table, name string, maximum driver.Value) companySQLStep {
	return companySQLStep{contains: []string{"information_schema.columns", "current_schema()"}, columns: []string{"character_maximum_length"}, values: []driver.Value{maximum}, args: []driver.Value{table, name}}
}

func TestCopiedTextMigrationWidensAtomicallyWithoutTruncation(t *testing.T) {
	base, boundary := lifecycleRepo(t,
		copiedColumn("leave_request", "leave_type", int64(50)),
		companySQLStep{contains: []string{"ALTER TABLE leave_request", "TYPE varchar(100)"}, excludes: []string{"USING", "DROP", "DELETE"}},
		copiedColumn("leave_balance", "leave_type", int64(50)),
		companySQLStep{contains: []string{"ALTER TABLE leave_balance", "TYPE varchar(100)"}, excludes: []string{"USING", "DROP", "DELETE"}},
		copiedColumn("payroll_transaction", "bank_name", int64(100)),
		companySQLStep{contains: []string{"ALTER TABLE payroll_transaction", "TYPE varchar(200)"}, excludes: []string{"USING", "DROP", "DELETE"}},
	)
	if err := migrations.WidenCopiedTextColumns(base.(*gormCompanyRepository).db); err != nil || !boundary.committed || boundary.rolledBack {
		t.Fatalf("migration transaction failed: %v", err)
	}
}

func TestCopiedTextMigrationDoesNotNarrowOrRewriteCurrentColumns(t *testing.T) {
	base, boundary := lifecycleRepo(t, copiedColumn("leave_request", "leave_type", int64(100)), copiedColumn("leave_balance", "leave_type", nil), copiedColumn("payroll_transaction", "bank_name", int64(255)))
	if err := migrations.WidenCopiedTextColumns(base.(*gormCompanyRepository).db); err != nil || !boundary.committed {
		t.Fatal(err)
	}
}

func TestCopiedTextMigrationRollsBackOnFailure(t *testing.T) {
	failure := errors.New("DDL failed")
	base, boundary := lifecycleRepo(t, copiedColumn("leave_request", "leave_type", int64(50)), companySQLStep{contains: []string{"ALTER TABLE leave_request"}, err: failure})
	if err := migrations.WidenCopiedTextColumns(base.(*gormCompanyRepository).db); !errors.Is(err, failure) || boundary.committed || !boundary.rolledBack {
		t.Fatalf("migration failed to roll back: %v", err)
	}
}

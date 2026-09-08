package service

import (
	"context"
	"testing"

	"payroll-backend/apperr"
	"payroll-backend/repository"
)

type companyAccessLeaseFake struct {
	states []repository.CompanyState
	owners map[repository.CompanyRecord]uint
	closed bool
}

func (l *companyAccessLeaseFake) States() []repository.CompanyState { return l.states }
func (l *companyAccessLeaseFake) RecordCompany(record repository.CompanyRecord) (uint, error) {
	id, ok := l.owners[record]
	if !ok {
		return 0, apperr.NotFound("Record not found")
	}
	return id, nil
}
func (l *companyAccessLeaseFake) Close() { l.closed = true }

type companyAccessRepoFake struct{ lease *companyAccessLeaseFake }

func (r companyAccessRepoFake) LockCompanies(context.Context, []uint) (repository.CompanyAccessLease, error) {
	return r.lease, nil
}

func TestCompanyAccessFiltersInactiveButKeepsOtherCompanies(t *testing.T) {
	lease := &companyAccessLeaseFake{states: []repository.CompanyState{{ID: 1, IsActive: false}, {ID: 2, IsActive: true}}}
	session, err := NewCompanyAccessService(companyAccessRepoFake{lease}).Begin(context.Background(), []uint{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	ids := session.ActiveCompanyIDs()
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("active companies = %v", ids)
	}
	if err := session.RequireCompany(2); err != nil {
		t.Fatal(err)
	}
	if err := session.RequireCompany(1); apperr.KindOf(err) != apperr.KindConflict || apperr.Code(err) != "COMPANY_INACTIVE" {
		t.Fatalf("inactive error = %v", err)
	}
	if err := session.RequireCompany(99); apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("unknown company error = %v", err)
	}
}

func TestCompanyAccessChecksStoredOwnerAndRejectsWholeBatch(t *testing.T) {
	active := repository.CompanyRecord{Kind: "employees", ID: 10}
	inactive := repository.CompanyRecord{Kind: "employees", ID: 20}
	lease := &companyAccessLeaseFake{
		states: []repository.CompanyState{{ID: 1, IsActive: true}, {ID: 2, IsActive: false}},
		owners: map[repository.CompanyRecord]uint{active: 1, inactive: 2},
	}
	session, err := NewCompanyAccessService(companyAccessRepoFake{lease}).Begin(context.Background(), []uint{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.RequireRecords([]repository.CompanyRecord{active, inactive}); apperr.Code(err) != "COMPANY_INACTIVE" {
		t.Fatalf("batch error = %v", err)
	}
	if err := session.RequireRecords([]repository.CompanyRecord{{Kind: "employees", ID: 99}}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("missing error = %v", err)
	}
	session.Close()
	if !lease.closed {
		t.Fatal("company locks were not released")
	}
}

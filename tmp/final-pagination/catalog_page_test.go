//go:build pagination_scratch

package repository

import (
	"database/sql/driver"
	"testing"

	"payroll-backend/models"
)

func TestCatalogPageKeepsCompanyScopeInCountAndRows(t *testing.T) {
	repo, _ := lifecycleRepo(t,
		companySQLStep{contains: []string{"count(*)", "companies_id IN"}, columns: []string{"count"}, values: []driver.Value{int64(40)}},
		companySQLStep{contains: []string{"companies_id IN", "ORDER BY", "LIMIT", "OFFSET"}, columns: []string{"id"}},
	)
	db := repo.(*gormCompanyRepository).db
	scope := AccessScope{AuthorizedCompanyIDs: []uint{7}}
	page, err := readCatalogPage[models.Department](db.Scopes(scope.ApplyCompany("department.companies_id")), PageQuery{Page: 2, PageSize: 25, Order: "asc"}, nil)
	if err != nil || page.Pagination.Total != 40 || page.Pagination.Page != 2 || page.Pagination.TotalPages != 2 {
		t.Fatalf("unexpected page: %+v, %v", page, err)
	}
}

func TestCatalogPageRejectsUntrustedSortBeforeSQL(t *testing.T) {
	repo, _ := lifecycleRepo(t)
	db := repo.(*gormCompanyRepository).db
	_, err := readCatalogPage[models.Department](db, PageQuery{Page: 1, PageSize: 25, Order: "asc", Sort: "id; DROP TABLE department"}, nil)
	if err == nil {
		t.Fatal("expected invalid sort to be rejected")
	}
}

package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// A scripted SQL boundary exercises the real GORM repository without a live database.
type companySQLStep struct {
	contains  []string
	excludes  []string
	columns   []string
	values    []driver.Value
	args      []driver.Value
	checkArgs func([]driver.NamedValue)
}
type companySQL struct {
	t                     *testing.T
	steps                 []companySQLStep
	committed, rolledBack bool
}

func (s *companySQL) Connect(context.Context) (driver.Conn, error) { return s, nil }
func (s *companySQL) Driver() driver.Driver                        { return s }
func (s *companySQL) Open(string) (driver.Conn, error)             { return s, nil }
func (s *companySQL) Close() error                                 { return nil }
func (s *companySQL) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected prepared statement")
}
func (s *companySQL) Begin() (driver.Tx, error) { return s, nil }
func (s *companySQL) Commit() error             { s.committed = true; return nil }
func (s *companySQL) Rollback() error           { s.rolledBack = true; return nil }
func (s *companySQL) next(query string) companySQLStep {
	s.t.Helper()
	if len(s.steps) == 0 {
		s.t.Fatalf("unexpected SQL: %s", query)
	}
	step := s.steps[0]
	s.steps = s.steps[1:]
	for _, fragment := range step.contains {
		if !strings.Contains(query, fragment) {
			s.t.Errorf("SQL %s must contain %s", query, fragment)
		}
	}
	for _, fragment := range step.excludes {
		if strings.Contains(query, fragment) {
			s.t.Errorf("SQL %s must exclude %s", query, fragment)
		}
	}
	return step
}
func (s *companySQL) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	step := s.next(query)
	if step.checkArgs != nil {
		step.checkArgs(args)
	}
	if step.args != nil {
		actual := make([]driver.Value, len(args))
		for i, arg := range args {
			actual[i] = arg.Value
		}
		if !reflect.DeepEqual(actual, step.args) {
			s.t.Errorf("SQL args = %#v, want %#v", actual, step.args)
		}
	}
	return &companyRows{columns: step.columns, values: step.values}, nil
}
func (s *companySQL) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	s.next(query)
	return driver.RowsAffected(1), nil
}

type companyRows struct {
	columns []string
	values  []driver.Value
}

func (r *companyRows) Columns() []string { return r.columns }
func (r *companyRows) Close() error      { return nil }
func (r *companyRows) Next(dest []driver.Value) error {
	if r.values == nil {
		return io.EOF
	}
	copy(dest, r.values)
	r.values = nil
	return nil
}
func lifecycleRepo(t *testing.T, steps ...companySQLStep) (CompanyRepository, *companySQL) {
	t.Helper()
	boundary := &companySQL{t: t, steps: steps}
	conn := sql.OpenDB(boundary)
	t.Cleanup(func() {
		_ = conn.Close()
		if len(boundary.steps) != 0 {
			t.Errorf("%d SQL operations missing", len(boundary.steps))
		}
	})
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{NamingStrategy: schema.NamingStrategy{SingularTable: true}, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return NewCompanyRepository(db), boundary
}
func lockedCompany(active bool) companySQLStep {
	return companySQLStep{contains: []string{`FROM "companies"`, `"companies"."deleted_at" =`, `FOR UPDATE`}, columns: []string{"id", "name", "is_active", "deleted_at"}, values: []driver.Value{int64(7), "Existing", active, int64(0)}}
}
func cycleCount(count int64) companySQLStep {
	return companySQLStep{contains: []string{`FROM "payroll_cycle"`, `companies_id =`, `is_closed =`, `"payroll_cycle"."deleted_at" =`}, columns: []string{"count"}, values: []driver.Value{count}, args: []driver.Value{int64(7), false, int64(0)}}
}
func TestCompanyDeactivationRejectsOpenPayroll(t *testing.T) {
	repo, db := lifecycleRepo(t, lockedCompany(true), cycleCount(1))
	_, err := repo.SetActive(7, false, nil)
	if apperr.KindOf(err) != apperr.KindConflict || !db.rolledBack || db.committed {
		t.Fatalf("expected conflict and rollback: %v, %+v", err, db)
	}
}
func TestCompanyStatusTransitionsAndIdempotence(t *testing.T) {
	for _, tc := range []struct {
		name            string
		initial, target bool
	}{
		{"deactivate", true, false}, {"reactivate", false, true}, {"already inactive", false, false}, {"already active", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := []companySQLStep{lockedCompany(tc.initial)}
			if tc.initial != tc.target {
				if !tc.target {
					steps = append(steps, cycleCount(0))
				}
				steps = append(steps, companySQLStep{contains: []string{`UPDATE "companies" SET "is_active"=`, `"companies"."deleted_at" =`}, excludes: []string{`"deleted_at"=`, `"name"=`}})
			}
			repo, db := lifecycleRepo(t, steps...)
			company, err := repo.SetActive(7, tc.target, nil)
			if err != nil || company.IsActive != tc.target || company.ID != 7 || !db.committed {
				t.Fatalf("unexpected transition result: %+v, %v", company, err)
			}
		})
	}
}
func TestCompanyProfileCannotReactivateStaleCompany(t *testing.T) {
	repo, db := lifecycleRepo(t, lockedCompany(false))
	company := models.Companies{ID: 7, Name: "Stale update", IsActive: true}
	err := repo.Save(&company, nil)
	if apperr.KindOf(err) != apperr.KindConflict || !db.rolledBack {
		t.Fatalf("inactive profile update must fail: %v", err)
	}
}
func TestCompanyProfileWritesOnlyProfileColumns(t *testing.T) {
	repo, db := lifecycleRepo(t, lockedCompany(true), companySQLStep{
		contains: []string{`UPDATE "companies" SET`, `"name"=`, `"tax_id"=`, `"address"=`, `"phone"=`},
		excludes: []string{`"is_active"=`, `"deleted_at"=`, `"created_at"=`},
	})
	company := models.Companies{ID: 7, Name: "New name", IsActive: false}
	err := repo.Save(&company, nil)
	if err != nil || !company.IsActive || company.Name != "New name" || !db.committed {
		t.Fatalf("profile result: %+v, %v", company, err)
	}
}
func TestCompanyStatusMissingOrDeletedIsNotFound(t *testing.T) {
	step := lockedCompany(false)
	step.values = nil
	repo, db := lifecycleRepo(t, step)
	_, err := repo.SetActive(7, true, nil)
	if apperr.KindOf(err) != apperr.KindNotFound || !db.rolledBack {
		t.Fatalf("missing/deleted company should be hidden: %v", err)
	}
}

func TestCompanyManagementIncludesInactiveButOptionsFilterActive(t *testing.T) {
	t.Run("management", func(t *testing.T) {
		repo, _ := lifecycleRepo(t, companySQLStep{
			contains: []string{`FROM "companies"`, `"companies"."deleted_at" =`},
			excludes: []string{"is_active ="},
			columns:  []string{"id", "is_active"}, values: []driver.Value{int64(7), false},
		})
		companies, err := repo.ListAll()
		if err != nil || len(companies) != 1 || companies[0].IsActive {
			t.Fatalf("inactive company must remain manageable: %+v, %v", companies, err)
		}
	})
	t.Run("options", func(t *testing.T) {
		repo, _ := lifecycleRepo(t,
			companySQLStep{contains: []string{`FROM "employees"`}, columns: []string{"id", "companies_id"}},
			companySQLStep{contains: []string{"companies.is_active =", `"companies"."deleted_at" =`}, columns: []string{"id"}},
		)
		companies, err := repo.ListAccessible(1)
		if err != nil || len(companies) != 0 {
			t.Fatalf("options: %+v, %v", companies, err)
		}
	})
}

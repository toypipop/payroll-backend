package test

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"payroll-backend/handlers/employees"
	"payroll-backend/internal/testfixture"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// Only persistence is replaced; requests exercise the real handler and service.
type passwordRepo struct {
	repository.EmployeesRepository
	row models.Employees
}

func (r *passwordRepo) WithinTx(fn func(repository.EmployeesRepository) error) error { return fn(r) }
func (r *passwordRepo) ValidateCompanyReferences(models.Employees) error             { return nil }
func (r *passwordRepo) FindRole(uint) (models.Roles, error) {
	return models.Roles{CompaniesID: 1}, nil
}
func (r *passwordRepo) Create(row *models.Employees) error    { r.row = *row; return nil }
func (r *passwordRepo) AppendAuditLog(*models.AuditLog) error { return nil }
func (r *passwordRepo) FindScoped(repository.AccessScope, uint) (models.Employees, error) {
	return r.row, nil
}
func (r *passwordRepo) FindDetail(uint) (models.Employees, error) { return r.row, nil }
func (r *passwordRepo) UpdateFields(_ *models.Employees, fields map[string]any) error {
	if password, ok := fields["password"].(string); ok {
		r.row.Password = password
	}
	return nil
}

func TestEmployeeWritesKeepPasswordPrivate(t *testing.T) {
	for _, method := range []string{"POST", "PUT"} {
		t.Run(method, func(t *testing.T) {
			repo := &passwordRepo{row: models.Employees{ID: 7, CompaniesID: 1, RolesID: 2, Password: "old-hash"}}
			h := &employees.Handler{Employees: service.NewEmployeesService(repo, testfixture.RoleAssignments(1))}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("scope_user", models.Employees{ID: 99, CompaniesID: 1})
			c.Set("data_scope", models.ScopeCompanies)
			c.Set("authorized_company_ids", []uint{1})
			c.Params = gin.Params{{Key: "id", Value: "7"}}
			c.Request = httptest.NewRequest(method, "/employees", strings.NewReader(`{"companies_id":1,"roles_id":2,"email":"test@example.invalid","employees_code":"E01","first_name":"Test","last_name":"User","password":"test-password","dept_id":3,"pos_id":4,"employment_type_id":5,"bank_id":6,"account_name":"Test","account_no":"123","is_active":false}`))
			c.Request.Header.Set("Content-Type", "application/json")
			wantStatus := 201
			if method == "POST" {
				h.CreateEmployees(c)
			} else {
				wantStatus = 200
				h.UpdateEmployees(c)
			}
			if w.Code != wantStatus {
				t.Fatalf("status = %d", w.Code)
			}
			if err := bcrypt.CompareHashAndPassword([]byte(repo.row.Password), []byte("test-password")); err != nil {
				t.Fatal("stored password does not verify")
			}
			assertNoPassword(t, w.Body.Bytes())
			if method == "POST" && (repo.row.IsActive || repo.row.DeptID != 3 || repo.row.PosID != 4 || repo.row.EmploymentTypeID == nil || *repo.row.EmploymentTypeID != 5 || repo.row.BankID == nil || *repo.row.BankID != 6 || repo.row.AccountNo != "123" || repo.row.AccountName != "Test" || repo.row.Email != "test@example.invalid" || repo.row.EmployeesCode != "E01" || repo.row.FirstName != "Test" || repo.row.LastName != "User") {
				t.Fatal("create lost supported form fields")
			}
		})
	}
}

func TestCreateEmployeeIgnoresServerOwnedFields(t *testing.T) {
	repo := &passwordRepo{}
	h := &employees.Handler{Employees: service.NewEmployeesService(repo, testfixture.RoleAssignments(1))}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("scope_user", models.Employees{ID: 99, CompaniesID: 1})
	c.Set("data_scope", models.ScopeCompanies)
	c.Set("authorized_company_ids", []uint{1})
	c.Request = httptest.NewRequest("POST", "/employees", strings.NewReader(`{"employees_code":"E01","email":"test@example.com","first_name":"Test","last_name":"User","dept_id":3,"pos_id":4,"companies_id":1,"roles_id":2,"password":"test-password","id":888,"created_at":"2020-01-01T00:00:00Z","company_access":[{"company_id":999}],"companies":{"id":999},"roles":{"id":999}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.CreateEmployees(c)
	if w.Code != 201 {
		t.Fatalf("status = %d", w.Code)
	}
	if repo.row.ID != 0 || !repo.row.CreatedAt.IsZero() || len(repo.row.CompanyAccess) != 0 || repo.row.Companies.ID != 0 || repo.row.Roles.ID != 0 {
		t.Fatal("client controlled server-owned fields or associations")
	}
	if !repo.row.IsActive {
		t.Fatal("omitted active flag should default to true")
	}
}

func TestEmployeeWritesRejectPasswordsOutsideAllowedLength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		message  string
	}{
		{name: "too short", password: "short", message: "Password must be at least 6 characters"},
		{name: "too long", password: strings.Repeat("a", 31), message: "Password must be at most 30 characters"},
	}

	for _, method := range []string{"POST", "PUT"} {
		for _, tt := range tests {
			t.Run(method+"/"+tt.name, func(t *testing.T) {
				repo := &passwordRepo{row: models.Employees{ID: 7, CompaniesID: 1, RolesID: 2, Password: "old-hash"}}
				h := &employees.Handler{Employees: service.NewEmployeesService(repo, testfixture.RoleAssignments(1))}
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Set("scope_user", models.Employees{ID: 99, CompaniesID: 1})
				c.Set("data_scope", models.ScopeCompanies)
				c.Set("authorized_company_ids", []uint{1})
				c.Params = gin.Params{{Key: "id", Value: "7"}}
				body := `{"employees_code":"E01","email":"test@example.com","first_name":"Test","last_name":"User","dept_id":3,"pos_id":4,"companies_id":1,"roles_id":2,"password":` + strconv.Quote(tt.password) + `}`
				c.Request = httptest.NewRequest(method, "/employees", strings.NewReader(body))
				c.Request.Header.Set("Content-Type", "application/json")
				if method == "POST" {
					h.CreateEmployees(c)
				} else {
					h.UpdateEmployees(c)
				}
				if w.Code != 400 || !strings.Contains(w.Body.String(), tt.message) {
					t.Fatalf("response = %d %s, want password validation error", w.Code, w.Body.String())
				}
			})
		}
	}
}

func TestEmployeeJSONHidesPasswordInNestedResponses(t *testing.T) {
	data, err := json.Marshal(map[string]any{"items": []models.Employees{{ID: 7, Password: "secret-hash"}}})
	if err != nil {
		t.Fatal(err)
	}
	assertNoPassword(t, data)
}

func assertNoPassword(t *testing.T, data []byte) {
	t.Helper()
	if strings.Contains(string(data), `"password"`) || strings.Contains(string(data), "secret-hash") || strings.Contains(string(data), "test-password") {
		t.Error("response exposes password data")
	}
}

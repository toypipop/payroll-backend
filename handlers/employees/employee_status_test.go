package employees

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"payroll-backend/internal/testfixture"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"
)

type createStatusRepository struct{ repository.EmployeesRepository }

func (r createStatusRepository) ValidateCompanyReferences(models.Employees) error { return nil }
func (r createStatusRepository) FindRole(uint) (models.Roles, error) {
	return models.Roles{CompaniesID: 7}, nil
}
func (r createStatusRepository) WithinTx(fn func(repository.EmployeesRepository) error) error {
	return fn(r)
}
func (r createStatusRepository) Create(employee *models.Employees) error { employee.ID = 9; return nil }
func (r createStatusRepository) AppendAuditLog(*models.AuditLog) error   { return nil }

func TestCreateEmployeeStatusDefaultAndExplicitFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		body   string
		active bool
	}{
		{`{"companies_id":7,"roles_id":1,"password":"secret"}`, true},
		{`{"companies_id":7,"roles_id":1,"password":"secret","is_active":false}`, false},
		{`{"companies_id":7,"roles_id":1,"password":"secret","is_active":true}`, true},
	} {
		t.Run(tc.body, func(t *testing.T) {
			h := &Handler{Employees: service.NewEmployeesService(createStatusRepository{}, testfixture.RoleAssignments(7))}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("scope_user", models.Employees{ID: 1})
				c.Set("data_scope", models.ScopeCompanies)
				c.Set("authorized_company_ids", []uint{7})
				c.Next()
			})
			router.POST("/employees", h.CreateEmployees)
			req := httptest.NewRequest(http.MethodPost, "/employees", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != http.StatusCreated {
				t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
			}
			var employee models.Employees
			if err := json.Unmarshal(res.Body.Bytes(), &employee); err != nil {
				t.Fatal(err)
			}
			if employee.IsActive != tc.active {
				t.Fatalf("is_active=%v want %v", employee.IsActive, tc.active)
			}
		})
	}
}

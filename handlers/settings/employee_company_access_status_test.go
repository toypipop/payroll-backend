package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"
)

type createAccessStatusRepo struct {
	repository.EmployeeCompanyAccessRepository
	mapping models.EmployeeCompanyAccess
}

func (r *createAccessStatusRepo) FindEmployeeBrief(uint) (models.Employees, error) {
	return models.Employees{ID: 9, CompaniesID: 7, IsActive: true}, nil
}
func (r *createAccessStatusRepo) Create(mapping *models.EmployeeCompanyAccess, _ *models.AuditLog) error {
	mapping.ID = 42
	r.mapping = *mapping
	return nil
}
func (r *createAccessStatusRepo) FindDetail(uint) (models.EmployeeCompanyAccess, error) {
	return r.mapping, nil
}

type createAccessRoleRepo struct{ repository.RoleRepository }

func (r createAccessRoleRepo) FindScoped(repository.AccessScope, uint) (models.Roles, error) {
	return models.Roles{ID: 2, CompaniesID: 7}, nil
}

func TestCreateCompanyAccessStatusDefaultAndExplicitFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		suffix string
		active bool
	}{{"", true}, {`,"is_active":false`, false}, {`,"is_active":true`, true}} {
		t.Run(tc.suffix, func(t *testing.T) {
			h := &Handler{EmployeeCompanyAccess: service.NewEmployeeCompanyAccessService(&createAccessStatusRepo{}, createAccessRoleRepo{})}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("scope_user", models.Employees{ID: 1})
				c.Set("data_scope", models.ScopeCompanies)
				c.Set("authorized_company_ids", []uint{7})
				c.Next()
			})
			router.POST("/employee-company-access", h.CreateEmployeeCompanyAccess)
			req := httptest.NewRequest(http.MethodPost, "/employee-company-access", strings.NewReader(`{"employee_id":9,"company_id":7,"role_id":2`+tc.suffix+`}`))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != http.StatusCreated {
				t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
			}
			var mapping models.EmployeeCompanyAccess
			if err := json.Unmarshal(res.Body.Bytes(), &mapping); err != nil {
				t.Fatal(err)
			}
			if mapping.IsActive != tc.active {
				t.Fatalf("is_active=%v want %v", mapping.IsActive, tc.active)
			}
		})
	}
}

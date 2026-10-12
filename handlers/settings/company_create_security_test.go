package settings

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"
	"strings"
	"testing"
)

type companyCreateSecurityRepo struct {
	repository.CompanyRepository
	created   bool
	bootstrap repository.TenantBootstrap
}

func (r *companyCreateSecurityRepo) CreateTenant(c *models.Companies, b repository.TenantBootstrap, _ *models.AuditLog) error {
	r.created = true
	r.bootstrap = b
	c.ID = 10
	return nil
}
func TestCompanyCreateRejectsInjectedAssociationsBeforeBootstrap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, field := range []string{`"employee_access":[{"employee_id":9,"role_id":99}]`, `"payroll_config":{"id":77}`, `"attendance_config":{"id":88}`, `"id":99`} {
		t.Run(field, func(t *testing.T) {
			repo := &companyCreateSecurityRepo{}
			h := &Handler{Company: service.NewCompanyService(repo)}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("scope_user", models.Employees{ID: 9})
			c.Request = httptest.NewRequest("POST", "/companies", strings.NewReader(`{"name":"New",`+field+`}`))
			h.CreateCompanies(c)
			if w.Code != 400 || repo.created {
				t.Fatalf("status=%d created=%v", w.Code, repo.created)
			}
		})
	}
	repo := &companyCreateSecurityRepo{}
	h := &Handler{Company: service.NewCompanyService(repo)}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("scope_user", models.Employees{ID: 9})
	c.Request = httptest.NewRequest("POST", "/companies", strings.NewReader(`{"name":"New","tax_id":"1234567890123","address":"Example","phone":"0812345678"}`))
	h.CreateCompanies(c)
	if w.Code != 201 || !repo.created || repo.bootstrap.CreatorID != 9 {
		t.Fatalf("bootstrap failed %d %s", w.Code, w.Body.String())
	}
}

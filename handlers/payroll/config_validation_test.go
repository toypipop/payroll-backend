package payroll

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"payroll-backend/models"
	"payroll-backend/service"
)

func TestPayrollInvalidConfigurationReturns400WithoutWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, fields := range []string{`"sso_rate":-1`, `"sso_rate":101`, `"sso_max_amount":100000000`, `"sso_min_salary":-1`} {
		t.Run(fields, func(t *testing.T) {
			repo := &payrollConfigWriteRepo{}
			h := &Handler{PayrollConfig: service.NewPayrollConfigService(repo)}
			res := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(res)
			c.Set("scope_user", models.Employees{ID: 9})
			c.Set("data_scope", models.ScopeCompanies)
			c.Set("authorized_company_ids", []uint{1})
			c.Request = httptest.NewRequest("POST", "/global", strings.NewReader(`{"companies_id":1,"tax_calculation":"Progressive",`+fields+`}`))
			h.UpdatePayrollConfig(c)
			if res.Code != 400 || repo.saved != nil || repo.audit != nil {
				t.Fatalf("invalid config saved: %d %s", res.Code, res.Body.String())
			}
		})
	}
}

package payroll

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

type payrollConfigWriteRepo struct {
	repository.PayrollConfigRepository
	saved *models.PayrollConfig
	audit *models.AuditLog
}

func (r *payrollConfigWriteRepo) UpsertForCompany(configuration *models.PayrollConfig, audit *models.AuditLog) error {
	r.saved, r.audit = configuration, audit
	return nil
}

func TestPayrollConfigRequestBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"normal", `{"companies_id":1,"sso_rate":3,"sso_max_amount":650,"sso_min_salary":1550,"tax_calculation":"Progressive"}`, http.StatusOK},
		{"nested company", `{"companies_id":1,"companies":{"id":2},"sso_rate":1}`, http.StatusBadRequest},
		{"nested company before scalar", `{"companies":{"id":2},"companies_id":1,"sso_rate":1}`, http.StatusBadRequest},
		{"model identity", `{"companies_id":1,"id":99}`, http.StatusBadRequest},
		{"metadata", `{"companies_id":1,"created_at":"2000-01-01T00:00:00Z"}`, http.StatusBadRequest},
		{"unauthorized company", `{"companies_id":2,"sso_rate":1}`, http.StatusForbidden},
		{"missing company", `{"sso_rate":1}`, http.StatusForbidden},
		{"wrong field type", `{"companies_id":"1"}`, http.StatusBadRequest},
		{"multiple JSON values", `{"companies_id":1} {"companies_id":2}`, http.StatusBadRequest},
		{"malformed JSON", `{"companies_id":1`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &payrollConfigWriteRepo{}
			handler := &Handler{PayrollConfig: service.NewPayrollConfigService(repo)}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("scope_user", models.Employees{ID: 9, CompaniesID: 1})
				c.Set("data_scope", models.ScopeCompanies)
				// Reachable company 2 does not grant permission to edit its config.
				c.Set("accessible_company_ids", []uint{1, 2})
				c.Set("authorized_company_ids", []uint{1})
			})
			router.POST("/global", handler.UpdatePayrollConfig)
			request := httptest.NewRequest(http.MethodPost, "/global", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status=%d, want %d: %s", response.Code, tc.status, response.Body.String())
			}
			if tc.status != http.StatusOK {
				if repo.saved != nil || repo.audit != nil {
					t.Fatal("invalid or unauthorized input reached persistence")
				}
				return
			}
			config := repo.saved
			if config == nil || config.CompaniesID != 1 || config.Companies.ID != 0 || config.ID != 0 || !config.CreatedAt.IsZero() || config.SsoRate != 3 || config.SsoMaxAmount != 650 || config.SsoMinSalary != 1550 || config.TaxCalculation != "Progressive" {
				t.Fatalf("unexpected saved config: %+v", config)
			}
			if repo.audit == nil || repo.audit.CompaniesID == nil || *repo.audit.CompaniesID != 1 {
				t.Fatalf("audit company does not match authorized write: %+v", repo.audit)
			}
		})
	}
}

package settings

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

type statusCompanyRepository struct {
	repository.CompanyRepository
}

func (r statusCompanyRepository) SetActive(id uint, active bool, _ *models.AuditLog) (models.Companies, error) {
	return models.Companies{ID: id, IsActive: active}, nil
}

type optionsCompanyRepository struct {
	repository.CompanyRepository
}

func (optionsCompanyRepository) ListAccessible(uint) ([]models.Companies, error) {
	return []models.Companies{{ID: 2, Name: "Tech Corp", IsActive: true, TaxID: "1111111111111", Address: "111 Sukhumvit Rd", Phone: "021111111"}}, nil
}

func TestCompanyOptionsReturnOnlyDropdownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{Company: service.NewCompanyService(optionsCompanyRepository{})}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", uint(1)); c.Next() })
	router.GET("/companies/options", handler.GetCompanies)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/companies/options", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	if got, want := strings.TrimSpace(response.Body.String()), `[{"id":2,"name":"Tech Corp"}]`; got != want {
		t.Fatalf("body %s, want %s", got, want)
	}
}

func TestCompanyStatusRequiresBooleanAndAcceptsFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{}`, http.StatusBadRequest},
		{`{"is_active":null}`, http.StatusBadRequest},
		{`{"is_active":"false"}`, http.StatusBadRequest},
		{`{"is_active":false}`, http.StatusOK},
		{`{"is_active":true}`, http.StatusOK},
	} {
		t.Run(tc.body, func(t *testing.T) {
			handler := &Handler{Company: service.NewCompanyService(statusCompanyRepository{})}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("scope_user", models.Employees{ID: 1})
				c.Set("data_scope", models.ScopeCompanies)
				c.Set("authorized_company_ids", []uint{7})
				c.Next()
			})
			router.PATCH("/companies/:id/status", handler.SetCompanyStatus)
			request := httptest.NewRequest(http.MethodPatch, "/companies/7/status", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", response.Code, tc.status, response.Body.String())
			}
			if tc.status == http.StatusOK && !strings.Contains(response.Body.String(), strings.TrimSuffix(strings.TrimPrefix(tc.body, "{"), "}")) {
				t.Fatalf("response status did not round-trip: %s", response.Body.String())
			}
		})
	}
}

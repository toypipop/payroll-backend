package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"payroll-backend/apperr"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"
)

type accessLeaseFake struct{ closed bool }

func (l *accessLeaseFake) States() []repository.CompanyState {
	return []repository.CompanyState{{ID: 1, IsActive: false}, {ID: 2, IsActive: true}}
}
func (l *accessLeaseFake) Close() { l.closed = true }
func (l *accessLeaseFake) RecordCompany(record repository.CompanyRecord) (uint, error) {
	if record.ID == 10 {
		return 1, nil
	}
	if record.ID == 20 {
		return 2, nil
	}
	return 0, apperr.NotFound("Record not found")
}

type accessRepoFake struct{ lease *accessLeaseFake }

func (r accessRepoFake) LockCompanies(context.Context, []uint) (repository.CompanyAccessLease, error) {
	return r.lease, nil
}

func TestInactiveCompanyRequestsNeverReachHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ method, route, url, body string }{
		{"POST", "/employees", "/employees", `{"companies_id":1}`},
		{"POST", "/employees", "/employees", `{"COMPANIES_ID":1}`},
		{"GET", "/employees", "/employees?companies_id=1", ""},
		{"PUT", "/employees/:id", "/employees/10", `{"companies_id":2}`},
		{"DELETE", "/employees/:id", "/employees/10", ""},
		{"GET", "/salaries/history/:employees_id", "/salaries/history/10", ""},
		{"POST", "/salaries/bulk", "/salaries/bulk", `{"targets":[{"employees_id":20},{"employees_id":10}]}`},
		{"PUT", "/leaves/approve/:id", "/leaves/approve/10", `{"status":"approved"}`},
		{"POST", "/permissions/:id", "/permissions/10", `{}`},
		{"POST", "/payroll/calculate/:cycle_id", "/payroll/calculate/10", ""},
		{"GET", "/reports/tax-forms/50tawi/:transaction_id", "/reports/tax-forms/50tawi/10", ""},
		{"GET", "/payroll/my-slips", "/payroll/my-slips", ""},
	} {
		t.Run(tc.method+tc.url, func(t *testing.T) {
			lease := &accessLeaseFake{}
			guard := &Guard{companies: service.NewCompanyAccessService(accessRepoFake{lease})}
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("accessible_company_ids", []uint{1, 2})
				c.Set("scope_user", models.Employees{CompaniesID: 1})
			})
			r.Use(guard.ActiveCompanies())
			r.Handle(tc.method, "/api/v1"+tc.route, func(c *gin.Context) { t.Error("inactive request reached domain handler"); c.Status(200) })
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, "/api/v1"+tc.url, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			if w.Code != 409 || !strings.Contains(w.Body.String(), "COMPANY_INACTIVE") {
				t.Fatalf("response = %d %s", w.Code, w.Body.String())
			}
			if !lease.closed {
				t.Fatal("lock lease leaked")
			}
		})
	}
}

func TestActiveRequestHasFilteredScopeAndIntactBody(t *testing.T) {
	lease := &accessLeaseFake{}
	guard := &Guard{companies: service.NewCompanyAccessService(accessRepoFake{lease})}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("accessible_company_ids", []uint{1, 2}) })
	r.Use(guard.ActiveCompanies())
	r.POST("/api/v1/employees", func(c *gin.Context) {
		ids := c.MustGet("accessible_company_ids").([]uint)
		if len(ids) != 1 || ids[0] != 2 {
			t.Errorf("scope = %v", ids)
		}
		var input struct {
			CompaniesID uint `json:"companies_id"`
		}
		if err := c.ShouldBindJSON(&input); err != nil || input.CompaniesID != 2 {
			t.Errorf("body lost: %v %+v", err, input)
		}
		c.Status(http.StatusCreated)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/employees", strings.NewReader(`{"companies_id":2}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("response = %d %s", w.Code, w.Body.String())
	}
}

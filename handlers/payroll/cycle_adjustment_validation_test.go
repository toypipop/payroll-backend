package payroll

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"
)

type invalidCycleEarningRepo struct {
	repository.CycleEarningRepository
}

func (r invalidCycleEarningRepo) WithinTx(fn func(repository.CycleEarningRepository, repository.PayrollRunRepository) error) error {
	return fn(r, nil)
}
func (invalidCycleEarningRepo) FindScoped(repository.AccessScope, uint) (models.PayrollCycleEarning, error) {
	return models.PayrollCycleEarning{ID: 1, CycleID: 4, CompaniesID: 7}, nil
}

type invalidCycleDeductionRepo struct {
	repository.CycleDeductionRepository
}

func (r invalidCycleDeductionRepo) WithinTx(fn func(repository.CycleDeductionRepository, repository.PayrollRunRepository) error) error {
	return fn(r, nil)
}
func (invalidCycleDeductionRepo) FindScoped(repository.AccessScope, uint) (models.PayrollCycleDeduction, error) {
	return models.PayrollCycleDeduction{ID: 1, CycleID: 4, CompaniesID: 7}, nil
}
func TestCycleAdjustmentValidationReturns400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{CycleEarning: service.NewCycleEarningService(invalidCycleEarningRepo{}, nil, nil, nil), CycleDeduction: service.NewCycleDeductionService(invalidCycleDeductionRepo{}, nil, nil, nil)}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("scope_user", models.Employees{ID: 1})
		c.Set("data_scope", models.ScopeCompanies)
		c.Set("authorized_company_ids", []uint{7})
		c.Next()
	})
	router.POST("/cycle-earnings", h.CreateCycleEarning)
	router.PUT("/cycle-earnings/:id", h.UpdateCycleEarning)
	router.POST("/cycle-deductions", h.CreateCycleDeduction)
	router.PUT("/cycle-deductions/:id", h.UpdateCycleDeduction)
	for _, endpoint := range []string{"/cycle-earnings", "/cycle-deductions"} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			bodies := []string{`{"cycle_id":4,"amount":-500}`, `{"cycle_id":4,"amount":0}`, `{"cycle_id":4,"amount":10000000000000}`, `{"cycle_id":4,"amount":1e309}`}
			if method == http.MethodPut {
				bodies = append(bodies, `{"cycle_id":8,"amount":100}`, `{"amount":100}`)
			}
			for _, body := range bodies {
				t.Run(endpoint+"/"+method+"/"+body, func(t *testing.T) {
					path := endpoint
					if method == http.MethodPut {
						path += "/1"
					}
					req := httptest.NewRequest(method, path, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					res := httptest.NewRecorder()
					router.ServeHTTP(res, req)
					if res.Code != 400 {
						t.Fatalf("got %d: %s", res.Code, res.Body.String())
					}
					if method == http.MethodPut && (strings.Contains(body, `"cycle_id":8`) || body == `{"amount":100}`) && !strings.Contains(res.Body.String(), "Payroll cycle cannot be changed") {
						t.Fatalf("wrong validation: %s", res.Body.String())
					}
				})
			}
		}
	}
}

package employees

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

type invalidAmountEarnings struct {
	repository.EmployeeEarningRepository
}

func (invalidAmountEarnings) FindScoped(repository.AccessScope, uint) (models.EmployeeEarning, error) {
	return models.EmployeeEarning{ID: 1, CompaniesID: 1}, nil
}

type invalidAmountDeductions struct {
	repository.EmployeeDeductionRepository
}

func (invalidAmountDeductions) FindScoped(repository.AccessScope, uint) (models.EmployeeDeduction, error) {
	return models.EmployeeDeduction{ID: 1, CompaniesID: 1}, nil
}

// Missing dependencies deliberately fail if invalid amounts reach target lookups
// or persistence. Requests exercise real JSON binding, services and HTTP mapping.
func TestAdjustmentAmountRequestsReturn400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{EmployeeEarning: service.NewEmployeeEarningService(invalidAmountEarnings{}, nil, nil), EmployeeDeduction: service.NewEmployeeDeductionService(invalidAmountDeductions{}, nil, nil)}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("scope_user", models.Employees{ID: 9})
		c.Set("data_scope", models.ScopeCompanies)
		c.Set("authorized_company_ids", []uint{1})
		c.Next()
	})
	router.POST("/employee-earnings", h.CreateEmployeeEarning)
	router.PUT("/employee-earnings/:id", h.UpdateEmployeeEarning)
	router.POST("/employee-deductions", h.CreateEmployeeDeduction)
	router.PUT("/employee-deductions/:id", h.UpdateEmployeeDeduction)
	for _, base := range []string{"/employee-earnings", "/employee-deductions"} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			for _, amount := range []string{"-500", "0", "0.0099", "10000000000000", "1e309", "null", "\"NaN\""} {
				t.Run(base+"/"+method+"/"+amount, func(t *testing.T) {
					path := base
					if method == http.MethodPut {
						path += "/1"
					}
					body := `{"companies_id":1,"employees_id":5,"earning_type_id":2,"deduction_type_id":2,"amount":` + amount + `}`
					req := httptest.NewRequest(method, path, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					res := httptest.NewRecorder()
					router.ServeHTTP(res, req)
					if res.Code != http.StatusBadRequest {
						t.Fatalf("got %d: %s", res.Code, res.Body.String())
					}
				})
			}
		}
	}
}

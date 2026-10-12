package attendance

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"
)

type validationAttendanceRepo struct {
	repository.AttendanceConfigRepository
	written bool
}

func (r *validationAttendanceRepo) UpsertForCompany(*models.AttendanceConfig, *models.AuditLog) error {
	r.written = true
	return nil
}
func TestAttendanceInvalidRatesReturn400WithoutWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, field := range []string{"late_rate_per_minute", "absent_rate_per_day", "overtime_rate_per_minute", "extra_rate_per_day"} {
		for _, value := range []string{"-1", "100000000", "1e309"} {
			t.Run(field+value, func(t *testing.T) {
				repo := &validationAttendanceRepo{}
				h := &Handler{AttendanceConfig: service.NewAttendanceConfigService(repo)}
				res := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(res)
				c.Set("scope_user", models.Employees{ID: 9})
				c.Set("data_scope", models.ScopeCompanies)
				c.Set("authorized_company_ids", []uint{1})
				c.Request = httptest.NewRequest("POST", "/attendance/config", strings.NewReader(`{"companies_id":1,"`+field+`":`+value+`}`))
				c.Request.Header.Set("Content-Type", "application/json")
				h.UpdateAttendanceConfig(c)
				if res.Code != 400 || repo.written {
					t.Fatalf("invalid config saved: %d %s", res.Code, res.Body.String())
				}
			})
		}
	}
}

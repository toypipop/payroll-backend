package attendance

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

type timeLogWriteRepo struct {
	repository.TimeLogRepository
	written *models.TimeLog
	audit   *models.AuditLog
}

func (r *timeLogWriteRepo) FindScoped(repository.AccessScope, uint) (models.TimeLog, error) {
	return models.TimeLog{ID: 3, CompaniesID: 1, EmployeesID: 9}, nil
}
func (r *timeLogWriteRepo) Create(log *models.TimeLog, audit *models.AuditLog) error {
	r.written, r.audit = log, audit
	return nil
}
func (r *timeLogWriteRepo) Save(log *models.TimeLog, audit *models.AuditLog) error {
	return r.Create(log, audit)
}

func (*timeLogWriteRepo) CountHolidaysOn(uint, string) (int64, error) { return 0, nil }
func (*timeLogWriteRepo) CountDayOffsOn(uint, int) (int64, error)     { return 0, nil }
func (*timeLogWriteRepo) FindAttendanceConfig(uint) (models.AttendanceConfig, error) {
	return models.AttendanceConfig{}, repository.ErrAttendanceConfigNotFound
}

type timeLogScopeEmployees struct{ repository.EmployeesRepository }

func (*timeLogScopeEmployees) FindBriefScoped(scope repository.AccessScope, id uint) (models.Employees, error) {
	if id != scope.User.ID {
		return models.Employees{}, repository.ErrEmployeeNotFound
	}
	return models.Employees{ID: 9, CompaniesID: 1, IsActive: true}, nil
}

// Exercise JSON binding and the real service: nested model fields must never reach persistence.
func TestTimeLogWritesIgnoreInjectedAssociationsAndMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, injected := range []bool{false, true} {
			name := method + "/normal"
			body := `{"companies_id":1,"employees_id":9,"work_location_id":4,"remark":"manual entry","check_in":"2026-10-08T08:00:00+07:00","check_out":"2026-10-08T17:00:00+07:00"}`
			if injected {
				name = method + "/injected"
				body = `{"companies_id":1,"employees_id":9,"work_location_id":4,"remark":"manual entry","check_in":"2026-10-08T08:00:00+07:00","check_out":"2026-10-08T17:00:00+07:00","id":999,"status":"holiday","created_at":"2000-01-01T00:00:00Z","employees":{"id":77,"company_access":[{"id":200}]},"work_location":{"id":999}}`
			}
			t.Run(name, func(t *testing.T) {
				repo := &timeLogWriteRepo{}
				h := &Handler{TimeLog: service.NewTimeLogService(repo, &timeLogScopeEmployees{})}
				router := gin.New()
				router.Use(func(c *gin.Context) {
					c.Set("scope_user", models.Employees{ID: 9, CompaniesID: 1})
					c.Set("data_scope", models.ScopeOwn)
					c.Set("authorized_company_ids", []uint{1})
				})
				router.POST("/timelogs", h.CreateTimeLog)
				router.PUT("/timelogs/:id", h.UpdateTimeLog)
				path, wantStatus, wantID := "/timelogs", http.StatusCreated, uint(0)
				if method == http.MethodPut {
					path, wantStatus, wantID = "/timelogs/3", http.StatusOK, 3
				}
				request := httptest.NewRequest(method, path, strings.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if response.Code != wantStatus {
					t.Fatalf("status=%d: %s", response.Code, response.Body.String())
				}
				log := repo.written
				if log == nil || log.ID != wantID || log.EmployeesID != 9 || log.CompaniesID != 1 || log.WorkLocationID == nil || *log.WorkLocationID != 4 || log.Remark != "manual entry" {
					t.Fatalf("unexpected persisted fields: %+v", log)
				}
				if log.CheckIn == nil || log.CheckOut == nil || log.CheckIn.Format("15:04") != "08:00" || log.CheckOut.Format("15:04") != "17:00" {
					t.Fatalf("timestamps did not survive binding: %+v", log)
				}
				if log.Employees.ID != 0 || len(log.Employees.CompanyAccess) != 0 || log.WorkLocation != nil || !log.CreatedAt.IsZero() || log.Status != models.StatusOnTime {
					t.Fatalf("injected fields reached persistence: %+v", log)
				}
				if repo.audit == nil {
					t.Fatal("missing audit")
				}
			})
		}
	}
}

func TestTimeLogCreateRejectsUnauthorizedScalarEmployee(t *testing.T) {
	repo := &timeLogWriteRepo{}
	svc := service.NewTimeLogService(repo, &timeLogScopeEmployees{})
	scope := repository.NewAccessScope(models.Employees{ID: 9, CompaniesID: 1}, models.ScopeOwn, []uint{1})
	location := uint(4)
	_, err := svc.Create(scope, service.TimeLogInput{CompaniesID: 1, EmployeesID: 77, WorkLocationID: &location})
	if err == nil || repo.written != nil {
		t.Fatalf("unauthorized employee accepted: %v", err)
	}
}

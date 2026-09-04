package service

import (
	"testing"
	"time"

	"payroll-backend/models"
	"payroll-backend/repository"
)

type bulkTimeLogRepo struct {
	holidays map[string]int64
	created  []models.TimeLog
}

func (r *bulkTimeLogRepo) ListScoped(repository.AccessScope) ([]models.TimeLog, error) {
	return nil, nil
}
func (r *bulkTimeLogRepo) FindScoped(repository.AccessScope, uint) (models.TimeLog, error) {
	return models.TimeLog{}, nil
}
func (r *bulkTimeLogRepo) Create(*models.TimeLog) error { return nil }
func (r *bulkTimeLogRepo) CreateMany(logs []models.TimeLog) error {
	r.created = append(r.created, logs...)
	return nil
}
func (r *bulkTimeLogRepo) Save(*models.TimeLog) error   { return nil }
func (r *bulkTimeLogRepo) Delete(*models.TimeLog) error { return nil }
func (r *bulkTimeLogRepo) CountForEmployeeBetween(uint, time.Time, time.Time) (int64, error) {
	return 0, nil
}
func (r *bulkTimeLogRepo) CountHolidaysOn(_ uint, date string) (int64, error) {
	return r.holidays[date], nil
}
func (r *bulkTimeLogRepo) CountDayOffsOn(uint, int) (int64, error) { return 0, nil }
func (r *bulkTimeLogRepo) FindAttendanceConfig(uint) (models.AttendanceConfig, error) {
	return models.AttendanceConfig{}, repository.ErrAttendanceConfigNotFound
}

type bulkTimeLogEmployees struct{ employee models.Employees }

func (r *bulkTimeLogEmployees) ListScoped(repository.AccessScope) ([]models.Employees, error) {
	return nil, nil
}
func (r *bulkTimeLogEmployees) FindScoped(repository.AccessScope, uint) (models.Employees, error) {
	return r.employee, nil
}
func (r *bulkTimeLogEmployees) FindBriefScoped(repository.AccessScope, uint) (models.Employees, error) {
	return r.employee, nil
}
func (r *bulkTimeLogEmployees) FindRole(uint) (models.Roles, error)                  { return models.Roles{}, nil }
func (r *bulkTimeLogEmployees) Create(*models.Employees) error                       { return nil }
func (r *bulkTimeLogEmployees) UpdateFields(*models.Employees, map[string]any) error { return nil }
func (r *bulkTimeLogEmployees) Delete(*models.Employees) error                       { return nil }
func (r *bulkTimeLogEmployees) FindDetail(uint) (models.Employees, error) {
	return models.Employees{}, nil
}

// Removing the holiday check would create a log for 2027-07-28, so this protects the bulk-generation rule.
func TestBulkCreateSkipsCompanyHoliday(t *testing.T) {
	repo := &bulkTimeLogRepo{holidays: map[string]int64{"2027-07-28": 1}}
	employees := &bulkTimeLogEmployees{employee: models.Employees{ID: 10, CompaniesID: 1}}
	service := NewTimeLogService(repo, employees)
	scope := repository.NewAccessScope(models.Employees{ID: 99, CompaniesID: 1}, models.ScopeCompanies, []uint{1})

	result, err := service.BulkCreate(scope, BulkTimeLogInput{
		EmployeesID: 10, CompaniesID: 1, WorkLocationID: 1,
		StartDate: "2027-07-26", EndDate: "2027-07-28", Weekdays: []int{1, 2, 3},
		CheckInTime: "09:00", CheckOutTime: "18:00",
	})
	if err != nil {
		t.Fatalf("BulkCreate returned error: %v", err)
	}
	if result.CreatedCount != 2 {
		t.Fatalf("created %d logs, want 2 after skipping the holiday", result.CreatedCount)
	}
	if len(result.HolidayDates) != 1 || result.HolidayDates[0] != "2027-07-28" {
		t.Fatalf("holiday dates = %v, want [2027-07-28]", result.HolidayDates)
	}
	if len(repo.created) != 2 {
		t.Fatalf("saved %d logs, want 2", len(repo.created))
	}
}

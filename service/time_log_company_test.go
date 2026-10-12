package service

import (
	"errors"
	"payroll-backend/models"
	"payroll-backend/repository"
	"testing"
)

type locationWriteRepo struct {
	bulkTimeLogRepo
	writes int
}

func (r *locationWriteRepo) FindScoped(repository.AccessScope, uint) (models.TimeLog, error) {
	return models.TimeLog{ID: 3, CompaniesID: 1, EmployeesID: 10}, nil
}
func (r *locationWriteRepo) Create(*models.TimeLog, *models.AuditLog) error { r.writes++; return nil }
func (r *locationWriteRepo) Save(*models.TimeLog, *models.AuditLog) error   { r.writes++; return nil }
func (r *locationWriteRepo) CreateMany(logs []models.TimeLog, a *models.AuditLog) error {
	r.writes++
	return r.bulkTimeLogRepo.CreateMany(logs, a)
}
func TestTimeLogLocationCompanyChecksEveryWritePath(t *testing.T) {
	for _, path := range []string{"create", "update", "bulk"} {
		for _, tc := range []struct {
			name    string
			company uint
			id      uint
			lookup  error
			want    error
		}{
			{"same company", 1, 4, nil, nil}, {"other company", 2, 4, nil, ErrTimeLogWorkLocationCompany}, {"missing", 1, 4, repository.ErrWorkLocationNotFound, repository.ErrWorkLocationNotFound}, {"zero", 1, 0, nil, ErrTimeLogWorkLocationRequired},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				r := &locationWriteRepo{}
				employees := &bulkTimeLogEmployees{employee: models.Employees{ID: 10, CompaniesID: 1, IsActive: true}}
				svc := NewTimeLogService(r, employees, timeLogLocations{companyID: tc.company, err: tc.lookup})
				scope := repository.NewAccessScope(models.Employees{ID: 9}, models.ScopeCompanies, []uint{1, 2})
				var err error
				if path == "bulk" {
					_, err = svc.BulkCreate(scope, BulkTimeLogInput{EmployeesID: 10, CompaniesID: 1, WorkLocationID: tc.id, StartDate: "2027-07-26", EndDate: "2027-07-26", Weekdays: []int{1}, CheckInTime: "09:00", CheckOutTime: "18:00"})
				} else {
					input := TimeLogInput{EmployeesID: 10, CompaniesID: 1, WorkLocationID: &tc.id}
					if path == "create" {
						_, err = svc.Create(scope, input)
					} else {
						_, err = svc.Update(scope, 3, input)
					}
				}
				if !errors.Is(err, tc.want) {
					t.Fatalf("got %v want %v", err, tc.want)
				}
				if (r.writes == 1) != (tc.want == nil) {
					t.Fatalf("writes=%d err=%v", r.writes, err)
				}
			})
		}
	}
}

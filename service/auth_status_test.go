package service

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"payroll-backend/models"
	"payroll-backend/repository"
)

type statusAuthRepo struct {
	repository.AuthRepository
	employee models.Employees
}

func (r *statusAuthRepo) FindByEmail(string) (models.Employees, error) { return r.employee, nil }
func (r *statusAuthRepo) FindEmployee(uint) (models.Employees, error)  { return r.employee, nil }

func TestInactiveEmployeeCannotAuthenticateOrReadIdentity(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repo := &statusAuthRepo{employee: models.Employees{ID: 4, Password: string(hash), IsActive: false}}
	svc := NewAuthService(repo)
	if _, err := svc.Authenticate("example@test.invalid", "test-password"); !errors.Is(err, ErrAuthInactive) {
		t.Fatalf("inactive login: %v", err)
	}
	if _, err := svc.IdentityByID(4); !errors.Is(err, ErrAuthInactive) {
		t.Fatalf("inactive identity: %v", err)
	}
	if _, err := svc.Authenticate("example@test.invalid", "wrong"); !errors.Is(err, ErrAuthPasswordWrong) {
		t.Fatalf("bad password: %v", err)
	}
	repo.employee.IsActive = true
	if employee, err := svc.Authenticate("example@test.invalid", "test-password"); err != nil || employee.ID != 4 {
		t.Fatalf("active login: %v", err)
	}
}

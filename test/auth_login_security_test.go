package test

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"payroll-backend/apperr"
	authhandler "payroll-backend/handlers/auth"
	"payroll-backend/models"
	"payroll-backend/repository"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type loginRepo struct {
	repository.AuthRepository
	employee models.Employees
	err      error
}

func (r loginRepo) FindByEmail(string) (models.Employees, error) { return r.employee, r.err }

func TestLoginRejectsUnknownEmailAndWrongPasswordIdentically(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), 12)
	if err != nil {
		t.Fatal(err)
	}
	var expected string
	for _, tc := range []struct {
		name string
		repo loginRepo
	}{
		{"wrong password", loginRepo{employee: models.Employees{ID: 1, Password: string(hash), IsActive: true}}},
		{"unknown email", loginRepo{err: repository.ErrAuthEmailUnknown}},
		{"wrapped unknown email", loginRepo{err: fmt.Errorf("lookup: %w", repository.ErrAuthEmailUnknown)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &authhandler.Handler{Auth: service.NewAuthService(tc.repo)}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.invalid","password":"wrong-password"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			h.Login(c)
			if w.Code != 401 {
				t.Fatalf("status = %d, want 401", w.Code)
			}
			if cookies := w.Result().Cookies(); len(cookies) != 0 {
				t.Fatal("rejected login issued a cookie")
			}
			if expected == "" {
				expected = w.Body.String()
			} else if w.Body.String() != expected {
				t.Error("login response reveals whether the account exists")
			}
			if !strings.Contains(w.Body.String(), "Invalid email or password") {
				t.Error("expected generic credential message")
			}
		})
	}
}

func TestAuthenticationPreservesSuccessfulLoginAndDatabaseErrors(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewAuthService(loginRepo{employee: models.Employees{ID: 7, Password: string(hash), IsActive: true}})
	employee, err := svc.Authenticate("user@example.invalid", "correct-password")
	if err != nil || employee.ID != 7 {
		t.Fatal("valid credentials rejected")
	}
	dbErr := errors.New("database unavailable")
	_, err = service.NewAuthService(loginRepo{err: dbErr}).Authenticate("user@example.invalid", "wrong-password")
	if !errors.Is(err, dbErr) || apperr.KindOf(err) != apperr.KindInternal {
		t.Fatal("database failure was disguised as invalid credentials")
	}
}

func TestLoginWarnsAfterFiveFailuresAndLimitsAtTen(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	h := &authhandler.Handler{
		Auth:          service.NewAuthService(loginRepo{employee: models.Employees{ID: 1, Password: string(hash), IsActive: true}}),
		LoginAttempts: service.NewLoginAttemptLimiter(),
	}

	for attempt := 1; attempt <= 10; attempt++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.invalid","password":"wrong-password"}`))
		c.Request.Header.Set("Content-Type", "application/json")
		h.Login(c)

		if attempt < 10 && w.Code != 401 {
			t.Fatalf("attempt %d: status = %d, want 401", attempt, w.Code)
		}
		if attempt == 5 && !strings.Contains(w.Body.String(), "5 unsuccessful attempts") {
			t.Fatalf("attempt 5: missing warning: %s", w.Body.String())
		}
		if attempt == 10 {
			if w.Code != 429 {
				t.Fatalf("attempt 10: status = %d, want 429", w.Code)
			}
			if w.Header().Get("Retry-After") != "900" {
				t.Fatalf("retry-after = %q, want 900", w.Header().Get("Retry-After"))
			}
		}
	}
}

// Measure the real bcrypt paths without a flaky wall-clock assertion in CI.
func BenchmarkRejectedLogin(b *testing.B) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), 12)
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		repo loginRepo
	}{
		{"wrong_password", loginRepo{employee: models.Employees{ID: 1, Password: string(hash), IsActive: true}}},
		{"unknown_email", loginRepo{err: repository.ErrAuthEmailUnknown}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			svc := service.NewAuthService(tc.repo)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := svc.Authenticate("user@example.invalid", "wrong-password"); err == nil {
					b.Fatal("invalid login accepted")
				}
			}
		})
	}
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	authhandler "payroll-backend/handlers/auth"
	"payroll-backend/models"
	"payroll-backend/repository"
)

type statusAccessRepo struct {
	repository.AccessRepository
	active      bool
	scopeReads  int
	accessReads int
}

func (r *statusAccessRepo) FindScopeUser(uint) (models.Employees, error) {
	r.scopeReads++
	return models.Employees{ID: 4, CompaniesID: 7, IsActive: r.active}, nil
}
func (r *statusAccessRepo) AccessibleCompanyIDs(uint) []uint { r.accessReads++; return []uint{7} }
func (r *statusAccessRepo) AccessibleRoleIDs(uint) []uint    { return []uint{2} }

func TestExistingCookieRejectedAfterEmployeeDeactivation(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-status-secret")
	repo := &statusAccessRepo{active: true}
	guard := NewGuard(repo, nil, nil)
	router := gin.New()
	calls := 0
	router.GET("/protected", guard.Auth(), func(c *gin.Context) { calls++; c.Status(http.StatusOK) })
	claims := authhandler.TokenClaims{UserID: 4, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-only-status-secret"))
	if err != nil {
		t.Fatal(err)
	}
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.AddCookie(&http.Cookie{Name: authhandler.GetAuthCookieName(), Value: token})
		result := httptest.NewRecorder()
		router.ServeHTTP(result, req)
		return result
	}
	if res := request(); res.Code != http.StatusOK {
		t.Fatalf("active request status=%d", res.Code)
	}
	repo.active = false
	res := request()
	if res.Code != http.StatusUnauthorized || calls != 1 || repo.scopeReads != 2 || repo.accessReads != 1 {
		t.Fatalf("status=%d handlerCalls=%d statusReads=%d accessReads=%d", res.Code, calls, repo.scopeReads, repo.accessReads)
	}
	cleared := false
	for _, cookie := range res.Result().Cookies() {
		if cookie.Name == authhandler.GetAuthCookieName() && cookie.MaxAge < 0 && cookie.HttpOnly {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("inactive session cookie was not cleared")
	}
}

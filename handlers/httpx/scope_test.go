package httpx

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"payroll-backend/models"
)

func TestRequestScopePreservesCompanyPermissionScopes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, value := range []any{map[uint]models.DataScope{1: models.ScopeCompanies, 2: models.ScopeOwn}, "malformed", map[uint]models.DataScope(nil)} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("scope_user", models.Employees{ID: 9})
		c.Set("data_scope", models.ScopeCompanies)
		c.Set("authorized_company_ids", []uint{1, 2})
		c.Set("company_data_scopes", value)
		scope, ok := RequestScope(c)
		if !ok || scope.CompanyDataScopes == nil {
			t.Fatal("permission grants were dropped, allowing fallback to the widest scope")
		}
		if grants, valid := value.(map[uint]models.DataScope); valid && grants != nil {
			if scope.CompanyDataScopes[2] != models.ScopeOwn {
				t.Fatal("Own permission widened")
			}
		} else if len(scope.CompanyDataScopes) != 0 {
			t.Fatal("malformed grants must fail closed")
		}
	}
}

package payroll

import (
	"net/http"

	"payroll-backend/handlers/httpx"

	"github.com/gin-gonic/gin"
)

type preCheckIssue struct {
	Type      string   `json:"type"`
	Message   string   `json:"message,omitempty"`
	Employees []string `json:"employees,omitempty"`
}

// PreCheckPayroll reports what would be skipped or missing if the cycle were run now.
func PreCheckPayroll(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	cycleID, ok := httpx.ParseID(c, "cycle_id", "payroll cycle")
	if !ok {
		return
	}

	found, err := payrollRunService().PreCheck(scope, cycleID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	issues := make([]preCheckIssue, 0, len(found))
	for _, issue := range found {
		issues = append(issues, preCheckIssue{
			Type:      issue.Type,
			Message:   issue.Message,
			Employees: issue.Employees,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": len(issues) == 0, "issues": issues})
}

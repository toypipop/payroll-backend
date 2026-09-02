package employees

import (
	"errors"
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/models"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

// GetSalaries returns all employees in scope with their salary structure (nil if not yet assigned).
func (h *Handler) GetSalaries(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	employees, err := h.SalariesStructure.ListEmployees(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, employees)
}

// GetSalaryHistory returns every salary record for one employee (current +
func (h *Handler) GetSalaryHistory(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	employeesID, ok := httpx.ParseID(c, "employees_id", "employee")
	if !ok {
		return
	}

	entries, err := h.SalariesStructure.History(scope, employeesID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type historyRow struct {
		models.SalariesStructure
		CanDelete bool `json:"can_delete"`
		// LockedBy names the closed cycles paid from this record, oldest first, so the screen can say why the
		// row is read-only instead of only that it is.
		LockedBy []string `json:"locked_by"`
	}
	rows := make([]historyRow, 0, len(entries))
	for _, entry := range entries {
		lockedBy := entry.LockedBy
		if lockedBy == nil {
			lockedBy = []string{}
		}
		rows = append(rows, historyRow{SalariesStructure: entry.Record, CanDelete: entry.CanDelete, LockedBy: lockedBy})
	}
	c.JSON(http.StatusOK, rows)
}

// GetSalaryAudit returns the change log behind one employee's pay.
func (h *Handler) GetSalaryAudit(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	employeesID, ok := httpx.ParseID(c, "employees_id", "employee")
	if !ok {
		return
	}

	entries, err := h.SalariesStructure.AuditTrail(scope, employeesID)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, entries)
}

// bulkRaiseRequest is one batch as the screen sends it.
type bulkRaiseRequest struct {
	Mode          string  `json:"mode"`
	Value         float64 `json:"value"`
	EffectiveDate string  `json:"effective_date"`
	Reason        string  `json:"reason"`
	Targets       []struct {
		EmployeesID uint    `json:"employees_id"`
		NewSalary   float64 `json:"new_salary"`
	} `json:"targets"`
}

func (r bulkRaiseRequest) toInput() service.BulkRaiseInput {
	targets := make([]service.BulkRaiseTarget, 0, len(r.Targets))
	for _, t := range r.Targets {
		targets = append(targets, service.BulkRaiseTarget{EmployeesID: t.EmployeesID, NewSalary: t.NewSalary})
	}
	return service.BulkRaiseInput{
		Mode:          service.BulkRaiseMode(r.Mode),
		Value:         r.Value,
		EffectiveDate: r.EffectiveDate,
		Reason:        r.Reason,
		Targets:       targets,
	}
}

// PreviewBulkRaise reports what a batch would do to each selected employee, and why any of them cannot be
// done. It writes nothing.
func (h *Handler) PreviewBulkRaise(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input bulkRaiseRequest
	if !httpx.BindJSON(c, &input) {
		return
	}

	preview, err := h.SalariesStructure.BulkRaisePlan(scope, input.toInput())
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}

// ApplyBulkRaise puts the whole batch on the timeline, or none of it. A refusal answers with the same preview
// shape, so the screen can point at the people who stopped it rather than only saying no.
func (h *Handler) ApplyBulkRaise(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input bulkRaiseRequest
	if !httpx.BindJSON(c, &input) {
		return
	}

	preview, err := h.SalariesStructure.BulkRaiseApply(scope, input.toInput())
	if err != nil {
		if errors.Is(err, service.ErrBulkBlocked) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "preview": preview})
			return
		}
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Salaries raised successfully",
		"preview": preview,
	})
}

// UpdateSalaryRecord moves one record along the timeline and re-prices it — the edit behind the history page's
// Edit button. A save is addressed by effective date, so this is the only way an existing date changes.
func (h *Handler) UpdateSalaryRecord(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "salary record")
	if !ok {
		return
	}

	var input struct {
		EffectiveDate string  `json:"effective_date"`
		BaseSalaries  float64 `json:"base_salaries"`
		Version       uint    `json:"version"`
		Reason        string  `json:"reason"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	if err := h.SalariesStructure.UpdateRecord(scope, id, service.SalaryRecordEdit{
		EffectiveDate: input.EffectiveDate,
		BaseSalaries:  input.BaseSalaries,
		Version:       input.Version,
		Reason:        input.Reason,
	}); err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Salary record updated successfully"})
}

// DeleteSalaryRecord erases a salary record, but only one that no closed cycle ever used.
func (h *Handler) DeleteSalaryRecord(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}
	id, ok := httpx.ParseID(c, "id", "salary record")
	if !ok {
		return
	}

	// A delete carries its reason in the body: it is the one salary write with no form behind it, and the
	// trail would be missing exactly the changes hardest to explain after the fact.
	var input struct {
		Reason string `json:"reason"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	if err := h.SalariesStructure.DeleteRecord(scope, id, input.Reason); err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Salary record deleted successfully"})
}

package attendance

import (
	"net/http"

	"payroll-backend/handlers/httpx"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetAttendanceConfigs(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	query, ok := httpx.ParsePage(c)
	if !ok {
		return
	}
	companies, err := h.AttendanceConfig.List(scope, query)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, companies)
}

// GetTimelogScheduleConfig returns a minimal, per-company work schedule for the Time Logs page.
func (h *Handler) GetTimelogScheduleConfig(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	schedules, err := h.AttendanceConfig.Schedules(scope)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}

	type scheduleItem struct {
		CompaniesID   uint   `json:"companies_id"`
		WorkStartTime string `json:"work_start_time"`
		WorkEndTime   string `json:"work_end_time"`
		IsConfigured  bool   `json:"is_configured"`
	}
	result := make([]scheduleItem, 0, len(schedules))
	for _, schedule := range schedules {
		result = append(result, scheduleItem{
			CompaniesID:   schedule.CompaniesID,
			WorkStartTime: schedule.WorkStartTime,
			WorkEndTime:   schedule.WorkEndTime,
			IsConfigured:  schedule.IsConfigured,
		})
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) UpdateAttendanceConfig(c *gin.Context) {
	scope, ok := httpx.RequestScope(c)
	if !ok {
		return
	}

	var input struct {
		CompaniesID               uint    `json:"companies_id"`
		LateRatePerMinute         float64 `json:"late_rate_per_minute"`
		AbsentRatePerDay          float64 `json:"absent_rate_per_day"`
		IsLateDeductionEnabled    bool    `json:"is_late_deduction_enabled"`
		IsAbsentDeductionEnabled  bool    `json:"is_absent_deduction_enabled"`
		WorkStartTime             string  `json:"work_start_time"`
		WorkEndTime               string  `json:"work_end_time"`
		OvertimeRatePerMinute     float64 `json:"overtime_rate_per_minute"`
		ExtraRatePerDay           float64 `json:"extra_rate_per_day"`
		IsOvertimeEarningsEnabled bool    `json:"is_overtime_earnings_enabled"`
		IsExtraEarningsEnabled    bool    `json:"is_extra_earnings_enabled"`
	}
	if !httpx.BindJSON(c, &input) {
		return
	}

	if _, err := h.AttendanceConfig.Save(scope, service.AttendanceConfigInput{
		CompaniesID:               input.CompaniesID,
		LateRatePerMinute:         input.LateRatePerMinute,
		AbsentRatePerDay:          input.AbsentRatePerDay,
		IsLateDeductionEnabled:    input.IsLateDeductionEnabled,
		IsAbsentDeductionEnabled:  input.IsAbsentDeductionEnabled,
		WorkStartTime:             input.WorkStartTime,
		WorkEndTime:               input.WorkEndTime,
		OvertimeRatePerMinute:     input.OvertimeRatePerMinute,
		ExtraRatePerDay:           input.ExtraRatePerDay,
		IsOvertimeEarningsEnabled: input.IsOvertimeEarningsEnabled,
		IsExtraEarningsEnabled:    input.IsExtraEarningsEnabled,
	}); err != nil {
		httpx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration saved", "data": input})
}

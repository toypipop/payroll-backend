package attendance

import "payroll-backend/service"

// Handler carries the services this package's routes need, built once in main so no handler reaches for a database handle of its own.
type Handler struct {
	AttendanceConfig  *service.AttendanceConfigService
	AttendanceSummary *service.AttendanceSummaryService
	LeaveRequest      *service.LeaveRequestService
	TimeLog           *service.TimeLogService
}

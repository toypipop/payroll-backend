package settings

import "payroll-backend/service"

// Handler carries the services this package's routes need, built once in main so no handler reaches for a database handle of its own.
type Handler struct {
	Bank                  *service.BankService
	Company               *service.CompanyService
	DayOff                *service.DayOffService
	Department            *service.DepartmentService
	DepartmentCategory    *service.DepartmentCategoryService
	EmployeeCompanyAccess *service.EmployeeCompanyAccessService
	Holiday               *service.HolidayService
	LeaveType             *service.LeaveTypeSettingService
	Permission            *service.PermissionService
	Position              *service.PositionService
	Role                  *service.RoleService
	WorkLocation          *service.WorkLocationService
}

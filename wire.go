package main

import (
	"log"

	atthandler "payroll-backend/handlers/attendance"
	authhandler "payroll-backend/handlers/auth"
	emphandler "payroll-backend/handlers/employees"
	payhandler "payroll-backend/handlers/payroll"
	cfghandler "payroll-backend/handlers/payrollconfig"
	rephandler "payroll-backend/handlers/reports"
	sethandler "payroll-backend/handlers/settings"
	"payroll-backend/middleware"
	"payroll-backend/repository"
	"payroll-backend/routes"
	"payroll-backend/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// buildGuard assembles the request-gating middleware from the lookups it needs.
func buildGuard(db *gorm.DB) *middleware.Guard {
	return middleware.NewGuard(
		repository.NewAccessRepository(db),
		repository.NewDeleteGuardRepository(db),
		service.NewCompanyAccessService(repository.NewCompanyAccessRepository(db)),
		service.NewSessionService(repository.NewSessionRepository(db)),
	)
}

// buildHandlers is the composition root: repositories and services are built here so nothing below main needs a database handle of its own.
func buildHandlers(db *gorm.DB) routes.Handlers {
	// A unique-constraint failure must reach the client as a 409, not the generic 500 every untyped error becomes.
	if err := repository.RegisterErrorTranslation(db); err != nil {
		log.Fatalf("Database error translation setup failed: %T", err)
	}

	// Repositories shared by more than one service are built once and reused.
	employeesRepo := repository.NewEmployeesRepository(db)
	sessionService := service.NewSessionService(repository.NewSessionRepository(db))
	earningTypeRepo := repository.NewEarningTypeRepository(db)
	deductionTypeRepo := repository.NewDeductionTypeRepository(db)
	payrollRunRepo := repository.NewPayrollRunRepository(db)
	payrollConfigRepo := repository.NewPayrollConfigRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	companyRepo := repository.NewCompanyRepository(db)
	roleAssignments := service.NewRoleAssignmentService(repository.NewAccessRepository(db))

	payrollConfigSvc := service.NewPayrollConfigService(payrollConfigRepo)

	return routes.Handlers{
		Attendance: &atthandler.Handler{
			AttendanceConfig:  service.NewAttendanceConfigService(repository.NewAttendanceConfigRepository(db)),
			AttendanceSummary: service.NewAttendanceSummaryService(repository.NewAttendanceSummaryRepository(db)),
			LeaveRequest: service.NewLeaveRequestService(
				repository.NewLeaveRequestRepository(db),
				employeesRepo,
				repository.NewLeaveTypeSettingRepository(db),
			),
			TimeLog: service.NewTimeLogService(repository.NewTimeLogRepository(db), employeesRepo, repository.NewWorkLocationRepository(db)),
		},
		Auth: &authhandler.Handler{
			Auth:          service.NewAuthService(repository.NewAuthRepository(db)),
			Sessions:      sessionService,
			LoginAttempts: service.NewLoginAttemptLimiter(),
		},
		Employees: &emphandler.Handler{
			Employees: service.NewEmployeesService(employeesRepo, roleAssignments, sessionService),
			EmployeeDeduction: service.NewEmployeeDeductionService(
				repository.NewEmployeeDeductionRepository(db),
				employeesRepo,
				deductionTypeRepo,
			),
			EmployeeEarning: service.NewEmployeeEarningService(
				repository.NewEmployeeEarningRepository(db),
				employeesRepo,
				earningTypeRepo,
			),
			SalariesStructure: service.NewSalariesStructureService(
				repository.NewSalariesStructureRepository(db),
				employeesRepo,
			),
		},
		Payroll: &payhandler.Handler{
			PayrollRun:    service.NewPayrollRunService(payrollRunRepo),
			PayrollConfig: payrollConfigSvc,
			CycleEarning: service.NewCycleEarningService(
				repository.NewCycleEarningRepository(db),
				payrollRunRepo,
				employeesRepo,
				earningTypeRepo,
			),
			CycleDeduction: service.NewCycleDeductionService(
				repository.NewCycleDeductionRepository(db),
				payrollRunRepo,
				employeesRepo,
				deductionTypeRepo,
			),
		},
		PayrollConfig: &cfghandler.Handler{
			PayrollConfig:  payrollConfigSvc,
			EarningType:    service.NewEarningTypeService(earningTypeRepo),
			DeductionType:  service.NewDeductionTypeService(deductionTypeRepo),
			EmploymentType: service.NewEmploymentTypeService(repository.NewEmploymentTypeRepository(db)),
		},
		Reports: &rephandler.Handler{
			Report: service.NewReportService(repository.NewReportRepository(db)),
		},
		Settings: &sethandler.Handler{
			AuditLog:              service.NewAuditLogService(repository.NewAuditLogRepository(db)),
			Bank:                  service.NewBankService(repository.NewBankRepository(db)),
			Company:               service.NewCompanyService(companyRepo),
			DayOff:                service.NewDayOffService(repository.NewDayOffRepository(db), companyRepo),
			Department:            service.NewDepartmentService(repository.NewDepartmentRepository(db)),
			DepartmentCategory:    service.NewDepartmentCategoryService(repository.NewDepartmentCategoryRepository(db)),
			EmployeeCompanyAccess: service.NewEmployeeCompanyAccessService(repository.NewEmployeeCompanyAccessRepository(db), roleRepo, roleAssignments),
			Holiday:               service.NewHolidayService(repository.NewHolidayRepository(db)),
			LeaveType:             service.NewLeaveTypeSettingService(repository.NewLeaveTypeSettingRepository(db)),
			Permission:            service.NewPermissionService(repository.NewPermissionRepository(db), roleRepo),
			Position:              service.NewPositionService(repository.NewPositionRepository(db)),
			Role:                  service.NewRoleService(roleRepo),
			WorkLocation:          service.NewWorkLocationService(repository.NewWorkLocationRepository(db)),
		},
	}
}

// buildDatabaseAdmission shares one request budget across every database-backed route.
func buildDatabaseAdmission(db *gorm.DB) gin.HandlerFunc {
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to obtain database pool: %T", err)
	}
	return middleware.DatabaseAdmission(sqlDB.Stats().MaxOpenConnections)
}

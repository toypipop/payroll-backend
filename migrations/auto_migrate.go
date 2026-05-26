package migrations

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

func AutoMigrateSchema(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Companies{},
		&models.Roles{},
		&models.AppModule{},
		&models.RolesModuleAccess{},
		&models.Department{},
		&models.Position{},
		&models.Employees{},
		&models.SalariesStructure{},
		&models.PayrollCycle{},
		&models.PayrollTransaction{},
		&models.PayrollTransactionDetail{},
		&models.TimeLog{},
		&models.LeaveRequest{},
		&models.LeaveBalance{},
		&models.PayrollConfig{},
		&models.EarningType{},
		&models.DeductionType{},
		&models.EmployeeEarning{},
		&models.EmployeeDeduction{},
		&models.PayrollCycleEarning{},
		&models.PayrollCycleDeduction{},
		&models.AttendanceConfig{},
		&models.Bank{},
		&models.EmploymentType{},
		&models.Holiday{},
		&models.DayOff{},
		&models.WorkLocation{},
		&models.LeaveTypeSetting{},
		&models.EarningCategory{},
		&models.DeductionCategory{},
		&models.EmployeeCompanyAccess{},
	)
}

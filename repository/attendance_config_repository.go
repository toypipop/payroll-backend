package repository

import (
	"errors"
	"time"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrAttendanceConfigNotFound means the company has no schedule configuration yet, which is a normal state rather than a failure: callers fall back to a default.
var ErrAttendanceConfigNotFound = apperr.NotFound("Attendance configuration not found")

// AttendanceConfigRepository is the persistence boundary for the attendance-config domain.
type AttendanceConfigRepository interface {
	ListCompaniesWithConfig(scope AccessScope) ([]models.Companies, error)
	ListCompaniesWithConfigPage(scope AccessScope, q PageQuery) (Page[models.Companies], error)
	UpsertForCompany(configuration *models.AttendanceConfig, audit *models.AuditLog) error
}

type gormAttendanceConfigRepository struct {
	db *gorm.DB
}

// NewAttendanceConfigRepository returns the GORM-backed implementation on the given connection.
func NewAttendanceConfigRepository(db *gorm.DB) AttendanceConfigRepository {
	return &gormAttendanceConfigRepository{db: db}
}

// ListCompaniesWithConfig reads companies rather than configs so a company that has not been configured yet still appears on the page.
func (r *gormAttendanceConfigRepository) ListCompaniesWithConfigPage(scope AccessScope, q PageQuery) (Page[models.Companies], error) {
	return readCatalogPage[models.Companies](r.db.Scopes(scope.ApplyCompany("companies.id")).Preload("AttendanceConfig"), q, nil)
}

// saveFields lists every column a save owns, as a map rather than a struct.
func saveFields(configuration *models.AttendanceConfig) map[string]any {
	return map[string]any{
		"late_rate_per_minute":         configuration.LateRatePerMinute,
		"absent_rate_per_day":          configuration.AbsentRatePerDay,
		"is_late_deduction_enabled":    configuration.IsLateDeductionEnabled,
		"is_absent_deduction_enabled":  configuration.IsAbsentDeductionEnabled,
		"work_start_time":              configuration.WorkStartTime,
		"work_end_time":                configuration.WorkEndTime,
		"overtime_rate_per_minute":     configuration.OvertimeRatePerMinute,
		"extra_rate_per_day":           configuration.ExtraRatePerDay,
		"is_overtime_earnings_enabled": configuration.IsOvertimeEarningsEnabled,
		"is_extra_earnings_enabled":    configuration.IsExtraEarningsEnabled,
	}
}

// UpsertForCompany writes the configuration, updating in place when the company already has one: companies_id is unique, so a company has exactly one.
func (r *gormAttendanceConfigRepository) UpsertForCompany(configuration *models.AttendanceConfig, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return (&gormAttendanceConfigRepository{db: tx}).upsertForCompany(configuration, audit)
	})
}

func (r *gormAttendanceConfigRepository) upsertForCompany(configuration *models.AttendanceConfig, audit *models.AuditLog) error {
	fields := saveFields(configuration)
	now := time.Now()

	var existing models.AttendanceConfig
	err := r.db.Where("companies_id = ?", configuration.CompaniesID).First(&existing).Error
	if err == nil {
		fields["updated_at"] = now
		if updateErr := r.db.Model(&existing).Updates(fields).Error; updateErr != nil {
			return updateErr
		}
		configuration.ID = existing.ID
		if audit != nil {
			return r.db.Create(audit).Error
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	fields["companies_id"] = configuration.CompaniesID
	fields["created_at"] = now
	fields["updated_at"] = now
	configuration.CreatedAt, configuration.UpdatedAt = now, now
	row := struct {
		models.AttendanceConfig  `gorm:"embedded"`
		IsLateDeductionEnabled   *bool `gorm:"column:is_late_deduction_enabled"`
		IsAbsentDeductionEnabled *bool `gorm:"column:is_absent_deduction_enabled"`
	}{AttendanceConfig: *configuration, IsLateDeductionEnabled: &configuration.IsLateDeductionEnabled, IsAbsentDeductionEnabled: &configuration.IsAbsentDeductionEnabled}
	if err := r.db.Table("attendance_config").Omit(clause.Associations).Create(&row).Error; err != nil {
		return err
	}
	*configuration = row.AttendanceConfig
	if audit != nil && audit.EntityID != nil {
		*audit.EntityID = configuration.ID
	}
	if audit != nil {
		return r.db.Create(audit).Error
	}
	return nil
}

// Complete schedule source for the timelog options endpoint.
func (r *gormAttendanceConfigRepository) ListCompaniesWithConfig(scope AccessScope) ([]models.Companies, error) {
	var companies []models.Companies
	err := r.db.Scopes(scope.ApplyCompany("companies.id")).Preload("AttendanceConfig").Order("companies.id ASC").Find(&companies).Error
	return companies, err
}

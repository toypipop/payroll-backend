package repository

import (
	"errors"

	"payroll-backend/apperr"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// ErrLeaveTypeNotFound covers both "no such row" and "out of the caller's scope".
var ErrLeaveTypeNotFound = apperr.NotFound("Leave type not found or unauthorized")

// LeaveTypeSettingRepository is the persistence boundary for the leave-type settings domain.
type LeaveTypeSettingRepository interface {
	ListScoped(scope AccessScope) ([]models.LeaveTypeSetting, error)
	ListMinimalScoped(scope AccessScope) ([]models.LeaveTypeSetting, error)
	FindScoped(scope AccessScope, id uint) (models.LeaveTypeSetting, error)
	Create(leaveType *models.LeaveTypeSetting) error
	Save(leaveType *models.LeaveTypeSetting) error
	Delete(leaveType *models.LeaveTypeSetting) error
}

type gormLeaveTypeSettingRepository struct {
	db *gorm.DB
}

// NewLeaveTypeSettingRepository returns the GORM-backed implementation on the given connection.
func NewLeaveTypeSettingRepository(db *gorm.DB) LeaveTypeSettingRepository {
	return &gormLeaveTypeSettingRepository{db: db}
}

// scoped filters by authorized company, the filter MasterLeaveTypePage declares as a company-level module.
func (r *gormLeaveTypeSettingRepository) scoped(scope AccessScope) *gorm.DB {
	return r.db.Scopes(scope.ApplyCompany("leave_type_setting.companies_id"))
}

func (r *gormLeaveTypeSettingRepository) ListScoped(scope AccessScope) ([]models.LeaveTypeSetting, error) {
	var leaveTypes []models.LeaveTypeSetting
	err := r.scoped(scope).Preload("Companies").Order("name").Find(&leaveTypes).Error
	return leaveTypes, err
}

// ListMinimalScoped skips the relations, for the dropdown options endpoint that needs id/name only.
func (r *gormLeaveTypeSettingRepository) ListMinimalScoped(scope AccessScope) ([]models.LeaveTypeSetting, error) {
	var leaveTypes []models.LeaveTypeSetting
	err := r.scoped(scope).Order("name").Find(&leaveTypes).Error
	return leaveTypes, err
}

func (r *gormLeaveTypeSettingRepository) FindScoped(scope AccessScope, id uint) (models.LeaveTypeSetting, error) {
	var leaveType models.LeaveTypeSetting
	if err := r.scoped(scope).Where("id = ?", id).First(&leaveType).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.LeaveTypeSetting{}, ErrLeaveTypeNotFound
		}
		return models.LeaveTypeSetting{}, err
	}
	return leaveType, nil
}

func (r *gormLeaveTypeSettingRepository) Create(leaveType *models.LeaveTypeSetting) error {
	return r.db.Create(leaveType).Error
}

func (r *gormLeaveTypeSettingRepository) Save(leaveType *models.LeaveTypeSetting) error {
	return r.db.Save(leaveType).Error
}

func (r *gormLeaveTypeSettingRepository) Delete(leaveType *models.LeaveTypeSetting) error {
	return r.db.Delete(leaveType).Error
}

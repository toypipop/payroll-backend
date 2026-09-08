package repository

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

// DepartmentCategoryRepository is the persistence boundary for the department-category list.
type DepartmentCategoryRepository interface {
	List() ([]models.DepartmentCategory, error)
	Create(category *models.DepartmentCategory, audit *models.AuditLog) error
}

type gormDepartmentCategoryRepository struct {
	db *gorm.DB
}

// NewDepartmentCategoryRepository returns the GORM-backed implementation on the given connection.
func NewDepartmentCategoryRepository(db *gorm.DB) DepartmentCategoryRepository {
	return &gormDepartmentCategoryRepository{db: db}
}

// List returns every category by name; the table is global so no scope filter applies here.
func (r *gormDepartmentCategoryRepository) List() ([]models.DepartmentCategory, error) {
	var categories []models.DepartmentCategory
	err := r.db.Order("name").Find(&categories).Error
	return categories, err
}

func (r *gormDepartmentCategoryRepository) Create(category *models.DepartmentCategory, audit *models.AuditLog) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(category).Error; err != nil {
			return err
		}
		if audit != nil && audit.EntityID != nil {
			*audit.EntityID = category.ID
		}
		if audit != nil {
			return tx.Create(audit).Error
		}
		return nil
	})
}

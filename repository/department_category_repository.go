package repository

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

// DepartmentCategoryRepository is the persistence boundary for the department-category list.
type DepartmentCategoryRepository interface {
	List() ([]models.DepartmentCategory, error)
	Create(category *models.DepartmentCategory) error
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

func (r *gormDepartmentCategoryRepository) Create(category *models.DepartmentCategory) error {
	return r.db.Create(category).Error
}

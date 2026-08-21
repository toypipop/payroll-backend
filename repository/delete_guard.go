package repository

import (
	"errors"
	"fmt"
	"payroll-backend/models"

	"gorm.io/gorm"
)

// DeleteGuardRepository reports whether a master-data row is still referenced, so a delete can be refused before it reaches the database.
type DeleteGuardRepository interface {
	CheckDeleteBlocked(entity string, id uint, companiesID uint) (bool, string, error)
}

type gormDeleteGuardRepository struct {
	db *gorm.DB
}

// NewDeleteGuardRepository returns the GORM-backed implementation on the given connection.
func NewDeleteGuardRepository(db *gorm.DB) DeleteGuardRepository {
	return &gormDeleteGuardRepository{db: db}
}

var (
	ErrDeleteBlocked = errors.New("delete blocked: record is referenced")
)

// CheckDeleteBlocked returns blocked status
func (r *gormDeleteGuardRepository) CheckDeleteBlocked(entity string, id uint, companiesID uint) (bool, string, error) {
	switch entity {
	case "earning_type":
		if ok, err := r.modelCountExists(&models.EmployeeEarning{}, "earning_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employee earnings", nil
		}
		if ok, err := r.modelCountExists(&models.PayrollCycleEarning{}, "earning_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by cycle earnings", nil
		}
		if ok, err := r.payrollTransactionDetailRefExists("earning", id, companiesID); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by payroll transactions", nil
		}
		return false, "", nil

	case "deduction_type":
		if ok, err := r.modelCountExists(&models.EmployeeDeduction{}, "deduction_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employee deductions", nil
		}
		if ok, err := r.modelCountExists(&models.PayrollCycleDeduction{}, "deduction_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by cycle deductions", nil
		}
		if ok, err := r.payrollTransactionDetailRefExists("deduction", id, companiesID); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by payroll transactions", nil
		}
		return false, "", nil

	case "employment_type":
		if ok, err := r.modelCountExists(&models.Employees{}, "employment_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employees", nil
		}
		return false, "", nil

	case "department":
		effectiveCompaniesID, err := r.resolveCompaniesIDForEntity(entity, id, companiesID)
		if err != nil {
			return false, "", err
		}
		if effectiveCompaniesID != 0 {
			if ok, err := r.modelCountExists(&models.Employees{}, "dept_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by employees", nil
			}
			return false, "", nil
		}
		if ok, err := r.modelCountExists(&models.Employees{}, "dept_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employees", nil
		}
		return false, "", nil

	case "position":
		effectiveCompaniesID, err := r.resolveCompaniesIDForEntity(entity, id, companiesID)
		if err != nil {
			return false, "", err
		}
		if effectiveCompaniesID != 0 {
			if ok, err := r.modelCountExists(&models.Employees{}, "pos_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by employees", nil
			}
			return false, "", nil
		}
		if ok, err := r.modelCountExists(&models.Employees{}, "pos_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employees", nil
		}
		return false, "", nil

	case "role":
		effectiveCompaniesID, err := r.resolveCompaniesIDForEntity(entity, id, companiesID)
		if err != nil {
			return false, "", err
		}
		if effectiveCompaniesID != 0 {
			if ok, err := r.modelCountExists(&models.Employees{}, "roles_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by employees", nil
			}
			if ok, err := r.modelCountExists(&models.RolesModuleAccess{}, "roles_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by role permissions", nil
			}
			return false, "", nil
		}
		if ok, err := r.modelCountExists(&models.Employees{}, "roles_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employees", nil
		}
		if ok, err := r.modelCountExists(&models.RolesModuleAccess{}, "roles_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by role permissions", nil
		}
		return false, "", nil

	case "companies":
		// Company with data is referenced
		if ok, err := r.modelCountExists(&models.Employees{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has employees", nil
		}
		if ok, err := r.modelCountExists(&models.PayrollCycle{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has payroll cycles", nil
		}
		if ok, err := r.modelCountExists(&models.Department{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has departments", nil
		}
		if ok, err := r.modelCountExists(&models.Position{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has positions", nil
		}
		if ok, err := r.modelCountExists(&models.Roles{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has roles", nil
		}
		if ok, err := r.modelCountExists(&models.RolesModuleAccess{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has permissions", nil
		}
		if ok, err := r.modelCountExists(&models.PayrollConfig{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has payroll settings", nil
		}
		if ok, err := r.modelCountExists(&models.AttendanceConfig{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has attendance settings", nil
		}
		if ok, err := r.modelCountExists(&models.Holiday{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has holiday calendar", nil
		}
		if ok, err := r.modelCountExists(&models.WorkLocation{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has work locations", nil
		}
		if ok, err := r.modelCountExists(&models.LeaveTypeSetting{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has leave types", nil
		}
		if ok, err := r.modelCountExists(&models.EarningType{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has earning types", nil
		}
		if ok, err := r.modelCountExists(&models.DeductionType{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has deduction types", nil
		}
		if ok, err := r.modelCountExists(&models.TimeLog{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has timelogs", nil
		}
		if ok, err := r.modelCountExists(&models.LeaveRequest{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has leave requests", nil
		}
		return false, "", nil

	case "bank":
		if ok, err := r.modelCountExists(&models.Employees{}, "bank_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employee bank accounts", nil
		}
		return false, "", nil

	case "holiday":
		// No direct FK references
		return false, "", nil

	case "work_location":
		effectiveCompaniesID, err := r.resolveCompaniesIDForEntity(entity, id, companiesID)
		if err != nil {
			return false, "", err
		}
		if effectiveCompaniesID != 0 {
			if ok, err := r.modelCountExists(&models.TimeLog{}, "work_location_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by timelogs", nil
			}
			return false, "", nil
		}
		if ok, err := r.modelCountExists(&models.TimeLog{}, "work_location_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by timelogs", nil
		}
		return false, "", nil

	case "leave_type":
		effectiveCompaniesID, err := r.resolveCompaniesIDForEntity(entity, id, companiesID)
		if err != nil {
			return false, "", err
		}
		if effectiveCompaniesID != 0 {
			if ok, err := r.modelCountExists(&models.LeaveRequest{}, "leave_type_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by leave requests", nil
			}
			return false, "", nil
		}
		if ok, err := r.modelCountExists(&models.LeaveRequest{}, "leave_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by leave requests", nil
		}
		return false, "", nil

	default:
		return false, "", fmt.Errorf("unsupported delete guard entity: %s", entity)
	}
}

func (r *gormDeleteGuardRepository) modelCountExists(model any, where string, args ...any) (bool, error) {
	var count int64
	if err := r.db.Model(model).Where(where, args...).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Check payroll detail references
func (r *gormDeleteGuardRepository) payrollTransactionDetailRefExists(refType string, typeID uint, companiesID uint) (bool, error) {
	var count int64

	tableCol := "ptd.earning_type_id"
	if refType == "deduction" {
		tableCol = "ptd.deduction_type_id"
	}

	q := r.db.Table("payroll_transaction_detail ptd").
		Joins("JOIN payroll_transaction pt ON pt.id = ptd.transaction_id").
		Joins("JOIN payroll_cycle pc ON pc.id = pt.cycle_id").
		Where(tableCol+" = ?", typeID).
		Where("ptd.deleted_at = 0 AND pt.deleted_at = 0 AND pc.deleted_at = 0")

	if companiesID != 0 {
		q = q.Where("pc.companies_id = ?", companiesID)
	}

	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *gormDeleteGuardRepository) resolveCompaniesIDForEntity(entity string, id uint, companiesID uint) (uint, error) {
	if companiesID != 0 {
		return companiesID, nil
	}

	switch entity {
	case "department":
		var dept models.Department
		err := r.db.Select("companies_id").First(&dept, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		return dept.CompaniesID, nil

	case "position":
		var pos models.Position
		err := r.db.Select("companies_id").First(&pos, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		return pos.CompaniesID, nil

	case "role":
		var role models.Roles
		err := r.db.Select("companies_id").First(&role, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		return role.CompaniesID, nil

	case "work_location":
		var loc models.WorkLocation
		err := r.db.Select("companies_id").First(&loc, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		return loc.CompaniesID, nil

	case "leave_type":
		var lt models.LeaveTypeSetting
		err := r.db.Select("companies_id").First(&lt, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		return lt.CompaniesID, nil

	default:
		return 0, nil
	}
}

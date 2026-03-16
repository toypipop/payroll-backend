package repository

import (
	"errors"
	"fmt"
	"payroll-backend/config"
	"payroll-backend/models"

	"gorm.io/gorm"
)

var (
	ErrDeleteBlocked = errors.New("delete blocked: record is referenced")
)

// CheckDeleteBlocked returns (blocked, reason, error).
//
// companiesID is used for multi-tenant reference checks. It may be 0; in that case this function
// attempts to resolve the company from the target record (when applicable) and otherwise falls
// back to unscoped reference checks.
func CheckDeleteBlocked(entity string, id uint, companiesID uint) (bool, string, error) {
	switch entity {
	case "earning_type":
		if ok, err := modelCountExists(&models.EmployeeEarning{}, "earning_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employee earnings", nil
		}
		if ok, err := modelCountExists(&models.PayrollCycleEarning{}, "earning_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by cycle earnings", nil
		}
		if ok, err := payrollTransactionDetailRefExists("earning", id, companiesID); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by payroll transactions", nil
		}
		return false, "", nil

	case "deduction_type":
		if ok, err := modelCountExists(&models.EmployeeDeduction{}, "deduction_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employee deductions", nil
		}
		if ok, err := modelCountExists(&models.PayrollCycleDeduction{}, "deduction_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by cycle deductions", nil
		}
		if ok, err := payrollTransactionDetailRefExists("deduction", id, companiesID); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by payroll transactions", nil
		}
		return false, "", nil

	case "earning_category":
		if ok, err := modelCountExists(&models.EarningType{}, "category_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by earning types", nil
		}
		return false, "", nil

	case "deduction_category":
		if ok, err := modelCountExists(&models.DeductionType{}, "category_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by deduction types", nil
		}
		return false, "", nil

	case "employment_type":
		if ok, err := modelCountExists(&models.Employees{}, "employment_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employees", nil
		}
		return false, "", nil

	case "department":
		effectiveCompaniesID, err := resolveCompaniesIDForEntity(entity, id, companiesID)
		if err != nil {
			return false, "", err
		}
		if effectiveCompaniesID != 0 {
			if ok, err := modelCountExists(&models.Employees{}, "dept_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by employees", nil
			}
			return false, "", nil
		}
		if ok, err := modelCountExists(&models.Employees{}, "dept_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employees", nil
		}
		return false, "", nil

	case "position":
		effectiveCompaniesID, err := resolveCompaniesIDForEntity(entity, id, companiesID)
		if err != nil {
			return false, "", err
		}
		if effectiveCompaniesID != 0 {
			if ok, err := modelCountExists(&models.Employees{}, "pos_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by employees", nil
			}
			return false, "", nil
		}
		if ok, err := modelCountExists(&models.Employees{}, "pos_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employees", nil
		}
		return false, "", nil

	case "role":
		effectiveCompaniesID, err := resolveCompaniesIDForEntity(entity, id, companiesID)
		if err != nil {
			return false, "", err
		}
		if effectiveCompaniesID != 0 {
			if ok, err := modelCountExists(&models.Employees{}, "roles_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by employees", nil
			}
			if ok, err := modelCountExists(&models.RolesModuleAccess{}, "roles_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by role permissions", nil
			}
			return false, "", nil
		}
		if ok, err := modelCountExists(&models.Employees{}, "roles_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employees", nil
		}
		if ok, err := modelCountExists(&models.RolesModuleAccess{}, "roles_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by role permissions", nil
		}
		return false, "", nil

	case "companies":
		// If a company has any domain data, treat it as referenced.
		if ok, err := modelCountExists(&models.Employees{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has employees", nil
		}
		if ok, err := modelCountExists(&models.PayrollCycle{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has payroll cycles", nil
		}
		if ok, err := modelCountExists(&models.Department{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has departments", nil
		}
		if ok, err := modelCountExists(&models.Position{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has positions", nil
		}
		if ok, err := modelCountExists(&models.Roles{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has roles", nil
		}
		if ok, err := modelCountExists(&models.RolesModuleAccess{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has permissions", nil
		}
		if ok, err := modelCountExists(&models.PayrollConfig{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has payroll settings", nil
		}
		if ok, err := modelCountExists(&models.AttendanceConfig{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has attendance settings", nil
		}
		if ok, err := modelCountExists(&models.Holiday{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has holiday calendar", nil
		}
		if ok, err := modelCountExists(&models.WorkLocation{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has work locations", nil
		}
		if ok, err := modelCountExists(&models.LeaveTypeSetting{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has leave types", nil
		}
		if ok, err := modelCountExists(&models.EarningType{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has earning types", nil
		}
		if ok, err := modelCountExists(&models.DeductionType{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has deduction types", nil
		}
		if ok, err := modelCountExists(&models.TimeLog{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has timelogs", nil
		}
		if ok, err := modelCountExists(&models.LeaveRequest{}, "companies_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "company has leave requests", nil
		}
		return false, "", nil

	case "bank":
		if ok, err := modelCountExists(&models.EmployeeBankAccount{}, "bank_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by employee bank accounts", nil
		}
		return false, "", nil

	case "holiday":
		// No direct FK references today.
		return false, "", nil

	case "work_location":
		effectiveCompaniesID, err := resolveCompaniesIDForEntity(entity, id, companiesID)
		if err != nil {
			return false, "", err
		}
		if effectiveCompaniesID != 0 {
			if ok, err := modelCountExists(&models.TimeLog{}, "work_location_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by timelogs", nil
			}
			return false, "", nil
		}
		if ok, err := modelCountExists(&models.TimeLog{}, "work_location_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by timelogs", nil
		}
		return false, "", nil

	case "leave_type":
		effectiveCompaniesID, err := resolveCompaniesIDForEntity(entity, id, companiesID)
		if err != nil {
			return false, "", err
		}
		if effectiveCompaniesID != 0 {
			if ok, err := modelCountExists(&models.LeaveRequest{}, "leave_type_id = ? AND companies_id = ?", id, effectiveCompaniesID); err != nil {
				return false, "", err
			} else if ok {
				return true, "referenced by leave requests", nil
			}
			return false, "", nil
		}
		if ok, err := modelCountExists(&models.LeaveRequest{}, "leave_type_id = ?", id); err != nil {
			return false, "", err
		} else if ok {
			return true, "referenced by leave requests", nil
		}
		return false, "", nil

	default:
		return false, "", fmt.Errorf("unsupported delete guard entity: %s", entity)
	}
}

func modelCountExists(model any, where string, args ...any) (bool, error) {
	var count int64
	if err := config.DB.Model(model).Where(where, args...).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// payrollTransactionDetailRefExists checks payroll transaction detail references.
// refType: "earning" or "deduction".
func payrollTransactionDetailRefExists(refType string, typeID uint, companiesID uint) (bool, error) {
	var count int64

	tableCol := "ptd.earning_type_id"
	if refType == "deduction" {
		tableCol = "ptd.deduction_type_id"
	}

	q := config.DB.Table("payroll_transaction_detail ptd").
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

func resolveCompaniesIDForEntity(entity string, id uint, companiesID uint) (uint, error) {
	if companiesID != 0 {
		return companiesID, nil
	}

	switch entity {
	case "department":
		var dept models.Department
		err := config.DB.Select("companies_id").First(&dept, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		return dept.CompaniesID, nil

	case "position":
		var pos models.Position
		err := config.DB.Select("companies_id").First(&pos, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		return pos.CompaniesID, nil

	case "role":
		var role models.Roles
		err := config.DB.Select("companies_id").First(&role, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		return role.CompaniesID, nil

	case "work_location":
		var loc models.WorkLocation
		err := config.DB.Select("companies_id").First(&loc, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, nil
			}
			return 0, err
		}
		return loc.CompaniesID, nil

	case "leave_type":
		var lt models.LeaveTypeSetting
		err := config.DB.Select("companies_id").First(&lt, id).Error
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

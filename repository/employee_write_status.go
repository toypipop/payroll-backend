package repository

import (
	"errors"
	"sort"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"payroll-backend/apperr"
	"payroll-backend/models"
)

var ErrEmployeeInactive = apperr.Conflict("This employee is inactive. Reactivate the employee before creating or updating their records.")

// lockActiveEmployees is a persistence precondition, in addition to service-level
// scope/status validation. Call only within the write transaction: FOR SHARE must
// survive until the write and its audit commit. Status UPDATE/DELETE then either
// finishes before this read (and is rejected), or waits until this write commits.
// Where payroll-cycle locks are needed, acquire those before employee locks.
func lockActiveEmployees(tx *gorm.DB, employeeIDs ...uint) error {
	ids := append([]uint(nil), employeeIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		if i > 0 && ids[i-1] == id {
			continue
		}
		if err := lockEmployeeStatus(tx, id, "SHARE"); err != nil {
			return err
		}
	}
	return nil
}

func lockEmployeeStatus(tx *gorm.DB, id uint, strength string) error {
	if id == 0 {
		return ErrEmployeeNotFound
	}
	var employee models.Employees
	err := tx.Select("id, is_active").Clauses(clause.Locking{Strength: strength}).First(&employee, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrEmployeeNotFound
	}
	if err != nil {
		return err
	}
	if !employee.IsActive {
		return ErrEmployeeInactive
	}
	return nil
}

// Updates may reassign a record; lock the validated destination, not its old owner.
func employeeWriteTarget(current uint, fields map[string]any, column string) (uint, error) {
	if value, exists := fields[column]; exists {
		id, ok := value.(uint)
		if !ok || id == 0 {
			return 0, apperr.Validation("Invalid employee ID")
		}
		return id, nil
	}
	return current, nil
}

func lockEmployeeUpdate(tx *gorm.DB, current uint, fields map[string]any, column string) error {
	id, err := employeeWriteTarget(current, fields, column)
	if err != nil {
		return err
	}
	return lockActiveEmployees(tx, id)
}

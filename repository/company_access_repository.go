package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"payroll-backend/apperr"
	"payroll-backend/models"
)

type CompanyState struct {
	ID       uint
	IsActive bool
}

// CompanyRecord names a server-selected resource kind, never a client SQL identifier.
type CompanyRecord struct {
	Kind string
	ID   uint
}

// CompanyAccessLease holds shared company locks for the lifetime of an API operation.
// Domain repositories retain their own transactions. A status transition takes an
// exclusive company lock and therefore cannot overtake an in-flight operation.
type CompanyAccessLease interface {
	States() []CompanyState
	RecordCompany(CompanyRecord) (uint, error)
	Close()
}

type CompanyAccessRepository interface {
	LockCompanies(context.Context, []uint) (CompanyAccessLease, error)
}

type gormCompanyAccessRepository struct{ db *gorm.DB }
type gormCompanyAccessLease struct {
	tx     *gorm.DB
	states []CompanyState
}

func NewCompanyAccessRepository(db *gorm.DB) CompanyAccessRepository {
	return &gormCompanyAccessRepository{db: db}
}

func (r *gormCompanyAccessRepository) LockCompanies(ctx context.Context, ids []uint) (CompanyAccessLease, error) {
	// A client disconnect must not release the lease while a domain transaction
	// (which may not use the request context) is still running. Cancel acquisition
	// queries with ctx, but release the transaction only when the handler returns.
	tx := r.db.WithContext(context.WithoutCancel(ctx)).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	lease := &gormCompanyAccessLease{tx: tx, states: []CompanyState{}}
	if len(ids) > 0 {
		err := tx.WithContext(ctx).Model(&models.Companies{}).Select("id, is_active").Where("id IN ?", ids).
			Order("id ASC").Clauses(clause.Locking{Strength: "SHARE"}).Find(&lease.states).Error
		if err != nil {
			lease.Close()
			return nil, err
		}
	}
	return lease, nil
}

func (l *gormCompanyAccessLease) States() []CompanyState { return l.states }
func (l *gormCompanyAccessLease) Close()                 { _ = l.tx.Rollback().Error }

func (l *gormCompanyAccessLease) RecordCompany(record CompanyRecord) (uint, error) {
	var model any
	column := "companies_id"
	switch record.Kind {
	case "employees":
		model = &models.Employees{}
	case "salaries":
		model = &models.SalariesStructure{}
	case "employee-earnings":
		model = &models.EmployeeEarning{}
	case "employee-deductions":
		model = &models.EmployeeDeduction{}
	case "timelogs":
		model = &models.TimeLog{}
	case "leaves":
		model = &models.LeaveRequest{}
	case "cycles":
		model = &models.PayrollCycle{}
	case "cycle-earnings":
		model = &models.PayrollCycleEarning{}
	case "cycle-deductions":
		model = &models.PayrollCycleDeduction{}
	case "earning-types":
		model = &models.EarningType{}
	case "deduction-types":
		model = &models.DeductionType{}
	case "employment-types":
		model = &models.EmploymentType{}
	case "departments":
		model = &models.Department{}
	case "positions":
		model = &models.Position{}
	case "roles":
		model = &models.Roles{}
	case "employee-company-access":
		model = &models.EmployeeCompanyAccess{}
		column = "company_id"
	case "holidays":
		model = &models.Holiday{}
	case "day-offs":
		model = &models.DayOff{}
	case "work-locations":
		model = &models.WorkLocation{}
	case "leave-types":
		model = &models.LeaveTypeSetting{}
	case "transactions":
		var transaction models.PayrollTransaction
		if err := l.tx.Select("id, cycle_id").First(&transaction, record.ID).Error; err != nil {
			return 0, companyRecordError(err)
		}
		return l.RecordCompany(CompanyRecord{Kind: "cycles", ID: transaction.CycleID})
	default:
		return 0, apperr.Internal("Company ownership resolver is not configured")
	}
	var row struct{ CompanyID uint }
	err := l.tx.Model(model).Select(column+" AS company_id").Where("id = ?", record.ID).Take(&row).Error
	if err != nil {
		return 0, companyRecordError(err)
	}
	return row.CompanyID, nil
}

func companyRecordError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound("Record not found or access denied")
	}
	return err
}

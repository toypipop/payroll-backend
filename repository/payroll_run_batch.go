package repository

import (
	"payroll-backend/models"
	"payroll-backend/timeutil"

	"gorm.io/gorm/clause"
)

// PayrollRunBatchSize bounds IN lists, insert parameters and attendance data held by a run.
const PayrollRunBatchSize = 500

// PayrollEmployeeInputs contains only one employee's data for the requested cycle.
// An absent map entry means no enabled salary applies, not a failed lookup.
type PayrollEmployeeInputs struct {
	Salary             models.SalariesStructure
	Prorated           bool
	EmployeeEarnings   []models.EmployeeEarning
	CycleEarnings      []models.PayrollCycleEarning
	EmployeeDeductions []models.EmployeeDeduction
	CycleDeductions    []models.PayrollCycleDeduction
	TimeLogs           []models.TimeLog
	Leaves             []models.LeaveRequest
}

type EmployeeSnapshot struct {
	EmployeeID uint
	Snapshot   TransactionSnapshot
	Details    []models.PayrollTransactionDetail
}

// LoadEmployeeInputs runs a fixed set of queries for a bounded batch of employees
// selected by ListActiveEmployees. The caller holds the cycle lock and transaction.
func (r *gormPayrollRunRepository) LoadEmployeeInputs(cycle models.PayrollCycle, employeeIDs []uint) (map[uint]*PayrollEmployeeInputs, error) {
	inputs := make(map[uint]*PayrollEmployeeInputs, len(employeeIDs))
	if len(employeeIDs) == 0 {
		return inputs, nil
	}
	var salaries []models.SalariesStructure
	err := r.db.Select("DISTINCT ON (employees_id) *").
		Where("employees_id IN ?", employeeIDs).Scopes(usableSalaryBy(cycle.StartDate)).
		Order("employees_id, effective_date DESC, created_at DESC, id DESC").Find(&salaries).Error
	if err != nil {
		return nil, err
	}
	for _, salary := range salaries {
		inputs[salary.EmployeesID] = &PayrollEmployeeInputs{Salary: salary}
	}
	missing := make([]uint, 0)
	for _, id := range employeeIDs {
		if inputs[id] == nil {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		salaries = nil
		err = r.db.Select("DISTINCT ON (employees_id) *").
			Where("employees_id IN ?", missing).Scopes(usableSalaryBy(cycle.EndDate)).
			Order("employees_id, effective_date ASC, created_at ASC, id ASC").Find(&salaries).Error
		if err != nil {
			return nil, err
		}
		for _, salary := range salaries {
			inputs[salary.EmployeesID] = &PayrollEmployeeInputs{Salary: salary, Prorated: true}
		}
	}
	paidIDs := make([]uint, 0, len(inputs))
	for _, id := range employeeIDs {
		if inputs[id] != nil {
			paidIDs = append(paidIDs, id)
		}
	}
	if len(paidIDs) == 0 {
		return inputs, nil
	}

	var earnings []models.EmployeeEarning
	if err := r.db.Preload("EarningType").Where("employees_id IN ?", paidIDs).Order("id").Find(&earnings).Error; err != nil {
		return nil, err
	}
	for _, row := range earnings {
		inputs[row.EmployeesID].EmployeeEarnings = append(inputs[row.EmployeesID].EmployeeEarnings, row)
	}
	var cycleEarnings []models.PayrollCycleEarning
	if err := r.db.Preload("EarningType").Where("employees_id IN ? AND cycle_id = ?", paidIDs, cycle.ID).Order("id").Find(&cycleEarnings).Error; err != nil {
		return nil, err
	}
	for _, row := range cycleEarnings {
		inputs[row.EmployeesID].CycleEarnings = append(inputs[row.EmployeesID].CycleEarnings, row)
	}
	var deductions []models.EmployeeDeduction
	if err := r.db.Preload("DeductionType").Where("employees_id IN ?", paidIDs).Order("id").Find(&deductions).Error; err != nil {
		return nil, err
	}
	for _, row := range deductions {
		inputs[row.EmployeesID].EmployeeDeductions = append(inputs[row.EmployeesID].EmployeeDeductions, row)
	}
	var cycleDeductions []models.PayrollCycleDeduction
	if err := r.db.Preload("DeductionType").Where("employees_id IN ? AND cycle_id = ?", paidIDs, cycle.ID).Order("id").Find(&cycleDeductions).Error; err != nil {
		return nil, err
	}
	for _, row := range cycleDeductions {
		inputs[row.EmployeesID].CycleDeductions = append(inputs[row.EmployeesID].CycleDeductions, row)
	}
	var logs []models.TimeLog
	endExclusive := timeutil.DayStart(cycle.EndDate).AddDate(0, 0, 1)
	if err := r.db.Where("employees_id IN ? AND check_in >= ? AND check_in < ? AND check_in IS NOT NULL AND check_out IS NOT NULL", paidIDs, cycle.StartDate, endExclusive).Order("id").Find(&logs).Error; err != nil {
		return nil, err
	}
	for _, row := range logs {
		inputs[row.EmployeesID].TimeLogs = append(inputs[row.EmployeesID].TimeLogs, row)
	}
	var leaves []models.LeaveRequest
	if err := r.db.Where("employees_id IN ? AND status = ? AND start_date <= ? AND end_date >= ?", paidIDs, models.LeaveStatusApproved, cycle.EndDate, cycle.StartDate).Order("id").Find(&leaves).Error; err != nil {
		return nil, err
	}
	for _, row := range leaves {
		inputs[row.EmployeesID].Leaves = append(inputs[row.EmployeesID].Leaves, row)
	}
	return inputs, nil
}

// SaveSnapshots upserts the batch, then replaces its details. RETURNING supplies
// both existing and new transaction IDs. Every operation uses the run transaction.
func (r *gormPayrollRunRepository) SaveSnapshots(cycleID uint, snapshots []EmployeeSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	transactions := make([]models.PayrollTransaction, len(snapshots))
	for i, item := range snapshots {
		transactions[i] = transactionFromSnapshot(cycleID, item.EmployeeID, item.Snapshot)
	}
	if err := r.db.Omit(clause.Associations).Clauses(snapshotConflict(), clause.Returning{}).
		CreateInBatches(&transactions, PayrollRunBatchSize).Error; err != nil {
		return err
	}
	ids := make([]uint, len(transactions))
	transactionIDs := make(map[uint]uint, len(transactions))
	for i, row := range transactions {
		ids[i] = row.ID
		transactionIDs[row.EmployeesID] = row.ID
	}
	if err := r.db.Where("transaction_id IN ?", ids).Delete(&models.PayrollTransactionDetail{}).Error; err != nil {
		return err
	}
	var details []models.PayrollTransactionDetail
	for _, item := range snapshots {
		for _, detail := range item.Details {
			detail.TransactionID = transactionIDs[item.EmployeeID]
			details = append(details, detail)
		}
	}
	if len(details) == 0 {
		return nil
	}
	return r.db.Omit(clause.Associations).CreateInBatches(&details, PayrollRunBatchSize).Error
}

func (r *gormPayrollRunRepository) DeleteTransactionsForEmployees(cycleID uint, employeeIDs []uint) error {
	if len(employeeIDs) == 0 {
		return nil
	}
	return r.deleteTransactions(r.db.Model(&models.PayrollTransaction{}).Where("cycle_id = ? AND employees_id IN ?", cycleID, employeeIDs))
}

func transactionFromSnapshot(cycleID uint, employeeID uint, snapshot TransactionSnapshot) models.PayrollTransaction {
	return models.PayrollTransaction{
		CycleID:             cycleID,
		EmployeesID:         employeeID,
		SalariesStructureID: snapshot.SalariesStructureID,
		SalariesAtTime:      snapshot.SalariesAtTime,
		TotalEarning:        snapshot.TotalEarning,
		TotalDeduction:      snapshot.TotalDeduction,
		SsoAmount:           snapshot.SsoAmount,
		TaxableIncome:       snapshot.TaxableIncome,
		TaxAmount:           snapshot.TaxAmount,
		NetPay:              snapshot.NetPay,
		PositionName:        snapshot.PositionName,
		DepartmentName:      snapshot.DepartmentName,
		EmploymentTypeName:  snapshot.EmploymentTypeName,
		BankName:            snapshot.BankName,
		BankAccountName:     snapshot.BankAccountName,
		BankAccountNo:       snapshot.BankAccountNo,
	}
}

func snapshotConflict() clause.OnConflict {
	return clause.OnConflict{
		Columns: []clause.Column{{Name: "cycle_id"}, {Name: "employees_id"}, {Name: "deleted_at"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"updated_at", "salaries_structure_id", "salaries_at_time",
			"total_earning", "total_deduction", "sso_amount", "taxable_income", "tax_amount", "net_pay",
			"position_name", "department_name", "employment_type_name",
			"bank_name", "bank_account_name", "bank_account_no",
		}),
	}
}

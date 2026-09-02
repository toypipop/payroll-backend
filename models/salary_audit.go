package models

import "time"

// SalaryAuditAction names what was done to a salary record. The set is closed: every write path in
// SalariesStructureService records exactly one of these.
type SalaryAuditAction string

const (
	SalaryAuditCreated       SalaryAuditAction = "created"
	SalaryAuditAmountChanged SalaryAuditAction = "amount_changed"
	SalaryAuditMoved         SalaryAuditAction = "moved"
	SalaryAuditSwitchedOff   SalaryAuditAction = "switched_off"
	SalaryAuditDeleted       SalaryAuditAction = "deleted"
)

// SalaryAudit is one entry in the trail of who changed an employee's pay, when, and why.
//
// Rows are append-only: nothing updates or deletes one, and there is no soft-delete column, because an
// audit entry that can be removed answers no question worth asking.
type SalaryAudit struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	CompaniesID uint `gorm:"not null;index" json:"companies_id"`
	EmployeesID uint `gorm:"not null;index" json:"employees_id"`
	// SalariesStructureID is the record that moved. It is nullable because a delete leaves nothing to point
	// at, and because a record removed later must not take its own history with it.
	SalariesStructureID *uint `json:"salaries_structure_id"`

	Action SalaryAuditAction `gorm:"type:varchar(32);not null" json:"action"`
	Reason string            `gorm:"type:text;not null" json:"reason"`

	// ActorID identifies the person; ActorName is stored beside it rather than joined, so an entry keeps
	// saying who made the change even after that person is renamed or removed. An audit trail that shifts
	// when the employee table shifts is not a record of what happened.
	ActorID   uint   `gorm:"not null;index" json:"actor_id"`
	ActorName string `gorm:"type:varchar(255);not null" json:"actor_name"`

	// What actually changed. Both sides are nullable: a create has no "from", a switch-off has neither.
	FromAmount *float64   `gorm:"type:decimal(15,2)" json:"from_amount"`
	ToAmount   *float64   `gorm:"type:decimal(15,2)" json:"to_amount"`
	FromDate   *time.Time `json:"from_date"`
	ToDate     *time.Time `json:"to_date"`
}

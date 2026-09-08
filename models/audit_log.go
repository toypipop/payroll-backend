package models

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// AuditEventType is a closed name for a successful, auditable business change.
type AuditEventType string

const (
	AuditEventCompanyCreated     AuditEventType = "company.created"
	AuditEventCompanyUpdated     AuditEventType = "company.updated"
	AuditEventCompanyActivated   AuditEventType = "company.activated"
	AuditEventCompanyDeactivated AuditEventType = "company.deactivated"

	AuditEventRoleCreated       AuditEventType = "role.created"
	AuditEventRoleUpdated       AuditEventType = "role.updated"
	AuditEventRoleDeleted       AuditEventType = "role.deleted"
	AuditEventPermissionUpdated AuditEventType = "permission.updated"

	AuditEventEmployeeCreated AuditEventType = "employee.created"
	AuditEventEmployeeUpdated AuditEventType = "employee.updated"
	AuditEventEmployeeDeleted AuditEventType = "employee.deleted"

	AuditEventPayrollCycleCreated AuditEventType = "payroll_cycle.created"
	AuditEventPayrollCycleUpdated AuditEventType = "payroll_cycle.updated"
	AuditEventPayrollCycleClosed  AuditEventType = "payroll_cycle.closed"
	AuditEventPayrollRunCompleted AuditEventType = "payroll_run.completed"

	AuditEventTimeLogCreated AuditEventType = "time_log.created"
	AuditEventTimeLogUpdated AuditEventType = "time_log.updated"
	AuditEventTimeLogDeleted AuditEventType = "time_log.deleted"
	AuditEventLeaveCreated   AuditEventType = "leave_request.created"
	AuditEventLeaveUpdated   AuditEventType = "leave_request.updated"
	AuditEventLeaveApproved  AuditEventType = "leave_request.approved"
	AuditEventLeaveRejected  AuditEventType = "leave_request.rejected"

	AuditEventLeaveTypeCreated     AuditEventType = "leave_type.created"
	AuditEventLeaveTypeUpdated     AuditEventType = "leave_type.updated"
	AuditEventLeaveTypeDeleted     AuditEventType = "leave_type.deleted"
	AuditEventDepartmentCreated    AuditEventType = "department.created"
	AuditEventDepartmentUpdated    AuditEventType = "department.updated"
	AuditEventDepartmentDeleted    AuditEventType = "department.deleted"
	AuditEventPositionCreated      AuditEventType = "position.created"
	AuditEventPositionUpdated      AuditEventType = "position.updated"
	AuditEventPositionDeleted      AuditEventType = "position.deleted"
	AuditEventEarningTypeCreated   AuditEventType = "earning_type.created"
	AuditEventEarningTypeUpdated   AuditEventType = "earning_type.updated"
	AuditEventEarningTypeDeleted   AuditEventType = "earning_type.deleted"
	AuditEventDeductionTypeCreated AuditEventType = "deduction_type.created"
	AuditEventDeductionTypeUpdated AuditEventType = "deduction_type.updated"
	AuditEventDeductionTypeDeleted AuditEventType = "deduction_type.deleted"
	AuditEventHolidayCreated       AuditEventType = "holiday.created"
	AuditEventHolidayUpdated       AuditEventType = "holiday.updated"
	AuditEventHolidayDeleted       AuditEventType = "holiday.deleted"
	AuditEventWorkLocationCreated  AuditEventType = "work_location.created"
	AuditEventWorkLocationUpdated  AuditEventType = "work_location.updated"
	AuditEventWorkLocationDeleted  AuditEventType = "work_location.deleted"

	AuditEventSalaryCreated     AuditEventType = "salary.created"
	AuditEventSalaryChanged     AuditEventType = "salary.amount_changed"
	AuditEventSalaryMoved       AuditEventType = "salary.moved"
	AuditEventSalaryDeactivated AuditEventType = "salary.deactivated"
	AuditEventSalaryDeleted     AuditEventType = "salary.deleted"
)

var auditEventTypes = map[AuditEventType]struct{}{
	AuditEventCompanyCreated: {}, AuditEventCompanyUpdated: {}, AuditEventCompanyActivated: {}, AuditEventCompanyDeactivated: {},
	AuditEventRoleCreated: {}, AuditEventRoleUpdated: {}, AuditEventRoleDeleted: {}, AuditEventPermissionUpdated: {},
	AuditEventEmployeeCreated: {}, AuditEventEmployeeUpdated: {}, AuditEventEmployeeDeleted: {},
	AuditEventPayrollCycleCreated: {}, AuditEventPayrollCycleUpdated: {}, AuditEventPayrollCycleClosed: {}, AuditEventPayrollRunCompleted: {},
	AuditEventTimeLogCreated: {}, AuditEventTimeLogUpdated: {}, AuditEventTimeLogDeleted: {}, AuditEventLeaveCreated: {}, AuditEventLeaveUpdated: {}, AuditEventLeaveApproved: {}, AuditEventLeaveRejected: {},
	AuditEventLeaveTypeCreated: {}, AuditEventLeaveTypeUpdated: {}, AuditEventLeaveTypeDeleted: {}, AuditEventDepartmentCreated: {}, AuditEventDepartmentUpdated: {}, AuditEventDepartmentDeleted: {},
	AuditEventPositionCreated: {}, AuditEventPositionUpdated: {}, AuditEventPositionDeleted: {}, AuditEventEarningTypeCreated: {}, AuditEventEarningTypeUpdated: {}, AuditEventEarningTypeDeleted: {},
	AuditEventDeductionTypeCreated: {}, AuditEventDeductionTypeUpdated: {}, AuditEventDeductionTypeDeleted: {}, AuditEventHolidayCreated: {}, AuditEventHolidayUpdated: {}, AuditEventHolidayDeleted: {},
	AuditEventWorkLocationCreated: {}, AuditEventWorkLocationUpdated: {}, AuditEventWorkLocationDeleted: {},
	AuditEventSalaryCreated: {}, AuditEventSalaryChanged: {}, AuditEventSalaryMoved: {}, AuditEventSalaryDeactivated: {}, AuditEventSalaryDeleted: {},
}

var errAuditSensitiveField = errors.New("audit data contains a sensitive field")

// AuditLog is an append-only record of one successful business mutation.
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OccurredAt time.Time `gorm:"not null;index" json:"occurred_at"`
	CreatedAt  time.Time `json:"created_at"`

	ActorUserID       *uint           `gorm:"index" json:"actor_user_id"`
	ActorNameSnapshot string          `gorm:"type:varchar(255);not null" json:"actor_name_snapshot"`
	CompaniesID       *uint           `gorm:"index" json:"companies_id"`
	EventType         AuditEventType  `gorm:"type:varchar(64);not null;index" json:"event_type"`
	EntityType        string          `gorm:"type:varchar(64);not null;index" json:"entity_type"`
	EntityID          *uint           `gorm:"index" json:"entity_id"`
	Summary           string          `gorm:"type:varchar(500);not null" json:"summary"`
	Reason            string          `gorm:"type:text" json:"reason"`
	BeforeData        json.RawMessage `gorm:"type:jsonb" json:"before_data"`
	AfterData         json.RawMessage `gorm:"type:jsonb" json:"after_data"`
}

// AuditLogInput provides validated, structured data to construct an audit event.
type AuditLogInput struct {
	OccurredAt        time.Time
	ActorUserID       *uint
	ActorNameSnapshot string
	CompaniesID       *uint
	EventType         AuditEventType
	EntityType        string
	EntityID          *uint
	Summary           string
	Reason            string
	BeforeData        map[string]any
	AfterData         map[string]any
}

// NewAuditLogInput validates and serializes event data before it reaches persistence.
func NewAuditLogInput(input AuditLogInput) (AuditLog, error) {
	if _, ok := auditEventTypes[input.EventType]; !ok {
		return AuditLog{}, errors.New("unknown audit event type")
	}
	before, err := marshalAuditData(input.BeforeData)
	if err != nil {
		return AuditLog{}, err
	}
	after, err := marshalAuditData(input.AfterData)
	if err != nil {
		return AuditLog{}, err
	}
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return AuditLog{
		OccurredAt: occurredAt, ActorUserID: input.ActorUserID, ActorNameSnapshot: strings.TrimSpace(input.ActorNameSnapshot),
		CompaniesID: input.CompaniesID, EventType: input.EventType, EntityType: strings.TrimSpace(input.EntityType), EntityID: input.EntityID,
		Summary: strings.TrimSpace(input.Summary), Reason: strings.TrimSpace(input.Reason), BeforeData: before, AfterData: after,
	}, nil
}

func marshalAuditData(data map[string]any) (json.RawMessage, error) {
	if len(data) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if containsSensitiveAuditField(data) {
		return nil, errAuditSensitiveField
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func containsSensitiveAuditField(data map[string]any) bool {
	for key, value := range data {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "password", "password_hash", "token", "access_token", "refresh_token", "cookie", "authorization", "secret":
			return true
		}
		if nested, ok := value.(map[string]any); ok && containsSensitiveAuditField(nested) {
			return true
		}
	}
	return false
}

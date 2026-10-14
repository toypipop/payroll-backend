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
	AuditEventLeaveDeleted   AuditEventType = "leave_request.deleted"
	AuditEventLeaveApproved  AuditEventType = "leave_request.approved"
	AuditEventLeaveRejected  AuditEventType = "leave_request.rejected"

	AuditEventLeaveTypeCreated             AuditEventType = "leave_type.created"
	AuditEventLeaveTypeUpdated             AuditEventType = "leave_type.updated"
	AuditEventLeaveTypeDeleted             AuditEventType = "leave_type.deleted"
	AuditEventDepartmentCreated            AuditEventType = "department.created"
	AuditEventDepartmentUpdated            AuditEventType = "department.updated"
	AuditEventDepartmentDeleted            AuditEventType = "department.deleted"
	AuditEventPositionCreated              AuditEventType = "position.created"
	AuditEventPositionUpdated              AuditEventType = "position.updated"
	AuditEventPositionDeleted              AuditEventType = "position.deleted"
	AuditEventEarningTypeCreated           AuditEventType = "earning_type.created"
	AuditEventEarningTypeUpdated           AuditEventType = "earning_type.updated"
	AuditEventEarningTypeDeleted           AuditEventType = "earning_type.deleted"
	AuditEventDeductionTypeCreated         AuditEventType = "deduction_type.created"
	AuditEventDeductionTypeUpdated         AuditEventType = "deduction_type.updated"
	AuditEventDeductionTypeDeleted         AuditEventType = "deduction_type.deleted"
	AuditEventHolidayCreated               AuditEventType = "holiday.created"
	AuditEventHolidayUpdated               AuditEventType = "holiday.updated"
	AuditEventHolidayDeleted               AuditEventType = "holiday.deleted"
	AuditEventWorkLocationCreated          AuditEventType = "work_location.created"
	AuditEventWorkLocationUpdated          AuditEventType = "work_location.updated"
	AuditEventWorkLocationDeleted          AuditEventType = "work_location.deleted"
	AuditEventEmployeeEarningCreated       AuditEventType = "employee_earning.created"
	AuditEventEmployeeEarningUpdated       AuditEventType = "employee_earning.updated"
	AuditEventEmployeeEarningDeleted       AuditEventType = "employee_earning.deleted"
	AuditEventEmployeeDeductionCreated     AuditEventType = "employee_deduction.created"
	AuditEventEmployeeDeductionUpdated     AuditEventType = "employee_deduction.updated"
	AuditEventEmployeeDeductionDeleted     AuditEventType = "employee_deduction.deleted"
	AuditEventCycleEarningCreated          AuditEventType = "cycle_earning.created"
	AuditEventCycleEarningUpdated          AuditEventType = "cycle_earning.updated"
	AuditEventCycleEarningDeleted          AuditEventType = "cycle_earning.deleted"
	AuditEventCycleDeductionCreated        AuditEventType = "cycle_deduction.created"
	AuditEventCycleDeductionUpdated        AuditEventType = "cycle_deduction.updated"
	AuditEventCycleDeductionDeleted        AuditEventType = "cycle_deduction.deleted"
	AuditEventEmploymentTypeCreated        AuditEventType = "employment_type.created"
	AuditEventEmploymentTypeUpdated        AuditEventType = "employment_type.updated"
	AuditEventEmploymentTypeDeleted        AuditEventType = "employment_type.deleted"
	AuditEventDayOffCreated                AuditEventType = "day_off.created"
	AuditEventDayOffUpdated                AuditEventType = "day_off.updated"
	AuditEventDayOffDeleted                AuditEventType = "day_off.deleted"
	AuditEventAttendanceConfigSaved        AuditEventType = "attendance_config.saved"
	AuditEventPayrollConfigSaved           AuditEventType = "payroll_config.saved"
	AuditEventEmployeeCompanyAccessSaved   AuditEventType = "employee_company_access.saved"
	AuditEventEmployeeCompanyAccessDeleted AuditEventType = "employee_company_access.deleted"
	AuditEventDepartmentCategoryCreated    AuditEventType = "department_category.created"
	AuditEventBankCreated                  AuditEventType = "bank.created"
	AuditEventBankUpdated                  AuditEventType = "bank.updated"
	AuditEventBankDeleted                  AuditEventType = "bank.deleted"

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
	AuditEventTimeLogCreated: {}, AuditEventTimeLogUpdated: {}, AuditEventTimeLogDeleted: {}, AuditEventLeaveCreated: {}, AuditEventLeaveUpdated: {}, AuditEventLeaveDeleted: {}, AuditEventLeaveApproved: {}, AuditEventLeaveRejected: {},
	AuditEventLeaveTypeCreated: {}, AuditEventLeaveTypeUpdated: {}, AuditEventLeaveTypeDeleted: {}, AuditEventDepartmentCreated: {}, AuditEventDepartmentUpdated: {}, AuditEventDepartmentDeleted: {},
	AuditEventPositionCreated: {}, AuditEventPositionUpdated: {}, AuditEventPositionDeleted: {}, AuditEventEarningTypeCreated: {}, AuditEventEarningTypeUpdated: {}, AuditEventEarningTypeDeleted: {},
	AuditEventDeductionTypeCreated: {}, AuditEventDeductionTypeUpdated: {}, AuditEventDeductionTypeDeleted: {}, AuditEventHolidayCreated: {}, AuditEventHolidayUpdated: {}, AuditEventHolidayDeleted: {},
	AuditEventWorkLocationCreated: {}, AuditEventWorkLocationUpdated: {}, AuditEventWorkLocationDeleted: {},
	AuditEventEmployeeEarningCreated: {}, AuditEventEmployeeEarningUpdated: {}, AuditEventEmployeeEarningDeleted: {}, AuditEventEmployeeDeductionCreated: {}, AuditEventEmployeeDeductionUpdated: {}, AuditEventEmployeeDeductionDeleted: {},
	AuditEventCycleEarningCreated: {}, AuditEventCycleEarningUpdated: {}, AuditEventCycleEarningDeleted: {}, AuditEventCycleDeductionCreated: {}, AuditEventCycleDeductionUpdated: {}, AuditEventCycleDeductionDeleted: {},
	AuditEventEmploymentTypeCreated: {}, AuditEventEmploymentTypeUpdated: {}, AuditEventEmploymentTypeDeleted: {},
	AuditEventDayOffCreated: {}, AuditEventDayOffUpdated: {}, AuditEventDayOffDeleted: {},
	AuditEventAttendanceConfigSaved:      {},
	AuditEventPayrollConfigSaved:         {},
	AuditEventEmployeeCompanyAccessSaved: {}, AuditEventEmployeeCompanyAccessDeleted: {},
	AuditEventDepartmentCategoryCreated: {},
	AuditEventBankCreated:               {}, AuditEventBankUpdated: {}, AuditEventBankDeleted: {},
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
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	// Inspect the actual JSON shape: typed maps, structs, arrays and RawMessage
	// must not bypass the guard just because they are not map[string]any.
	var normalized any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, err
	}
	if containsSensitiveAuditValue(normalized) {
		return nil, errAuditSensitiveField
	}
	return encoded, nil
}

func containsSensitiveAuditValue(data any) bool {
	switch value := data.(type) {
	case []any:
		for _, item := range value {
			if containsSensitiveAuditValue(item) {
				return true
			}
		}
	case map[string]any:
		for key, item := range value {
			normalizedKey := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(key)))
			switch normalizedKey {
			case "password", "passwordhash", "token", "accesstoken", "refreshtoken", "authtoken", "jwt", "jwtsecret", "cookie", "setcookie", "authorization", "secret", "clientsecret", "apikey", "privatekey", "pgpassword", "sid", "sessionid", "credentials":
				return true
			}
			if containsSensitiveAuditValue(item) {
				return true
			}
		}
	}
	return false
}

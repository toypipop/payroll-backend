package migrations

import (
	"payroll-backend/models"

	"gorm.io/gorm"
)

const salaryAuditBackfillName = "20260908_backfill_salary_audit_to_audit_log"

// BackfillSalaryAudits moves the historical, salary-specific trail into the shared append-only audit log.
// The legacy table stays available until this migration has run in every deployed environment; keeping the
// conversion transactional means a failed deploy cannot leave a half-copied history behind.
func BackfillSalaryAudits(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.SalaryAudit{}) {
		return nil
	}
	return RunOnce(db, salaryAuditBackfillName, func(tx *gorm.DB) error {
		return tx.Transaction(func(tx *gorm.DB) error {
			return tx.Exec(`
				INSERT INTO audit_log (
					occurred_at, created_at, actor_user_id, actor_name_snapshot, companies_id,
					event_type, entity_type, entity_id, summary, reason, before_data, after_data
				)
				SELECT
					sa.created_at, sa.created_at, sa.actor_id, sa.actor_name, sa.companies_id,
					CASE sa.action
						WHEN 'created' THEN 'salary.created'
						WHEN 'amount_changed' THEN 'salary.amount_changed'
						WHEN 'moved' THEN 'salary.moved'
						WHEN 'switched_off' THEN 'salary.deactivated'
						WHEN 'deleted' THEN 'salary.deleted'
					END,
					'employee', sa.employees_id,
					CASE sa.action
						WHEN 'created' THEN 'Salary record created'
						WHEN 'amount_changed' THEN 'Salary amount changed'
						WHEN 'moved' THEN 'Salary record moved'
						WHEN 'switched_off' THEN 'Salary deactivated'
						WHEN 'deleted' THEN 'Salary record deleted'
					END,
					sa.reason,
					CASE WHEN sa.action IN ('amount_changed', 'moved', 'deleted')
						THEN jsonb_strip_nulls(jsonb_build_object(
							'base_salary', sa.from_amount, 'effective_date', to_char(sa.from_date, 'YYYY-MM-DD'),
							'salary_record_id', sa.salaries_structure_id
						))
						ELSE '{}'::jsonb END,
					CASE WHEN sa.action IN ('created', 'amount_changed', 'moved')
						THEN jsonb_strip_nulls(jsonb_build_object(
							'base_salary', sa.to_amount, 'effective_date', to_char(sa.to_date, 'YYYY-MM-DD'),
							'salary_record_id', sa.salaries_structure_id
						))
						ELSE '{}'::jsonb END
				FROM salary_audit AS sa
				WHERE sa.action IN ('created', 'amount_changed', 'moved', 'switched_off', 'deleted')
					AND NOT EXISTS (
						SELECT 1 FROM audit_log AS al
						WHERE al.entity_type = 'employee'
							AND al.entity_id = sa.employees_id
							AND al.occurred_at = sa.created_at
							AND al.event_type = CASE sa.action
								WHEN 'created' THEN 'salary.created'
								WHEN 'amount_changed' THEN 'salary.amount_changed'
								WHEN 'moved' THEN 'salary.moved'
								WHEN 'switched_off' THEN 'salary.deactivated'
								WHEN 'deleted' THEN 'salary.deleted'
							END
					)
			`).Error
		})
	})
}

package repository

import (
	"errors"

	"payroll-backend/apperr"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const (
	// uniqueViolation is the SQLSTATE Postgres raises when a write breaks a unique index.
	uniqueViolation = "23505"
	// foreignKeyViolation is the SQLSTATE Postgres raises when a write references a missing row or deletes a referenced row.
	foreignKeyViolation = "23503"
)

// TranslateUniqueViolation keeps the existing API while translating every client-correctable constraint failure.
func TranslateUniqueViolation(err error) error {
	return TranslateConstraintViolation(err)
}

// TranslateConstraintViolation turns client-correctable Postgres constraint failures into Conflict errors.
func TranslateConstraintViolation(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case uniqueViolation:
			return apperr.Conflict("A record with the same unique value already exists")
		case foreignKeyViolation:
			return apperr.Conflict("The requested relationship is invalid or this record is still in use")
		}
	}
	return err
}

// RegisterErrorTranslation makes every create, update, and delete on this connection report correctable constraint violations as Conflicts.
// It is a connection-level callback rather than a call in each repository because the mapping is a property of Postgres, not of any one domain, and a repository that forgets it is a silent 500.
func RegisterErrorTranslation(db *gorm.DB) error {
	translate := func(tx *gorm.DB) {
		tx.Error = TranslateConstraintViolation(tx.Error)
	}
	if err := db.Callback().Create().After("gorm:create").Register("apperr:unique_violation_create", translate); err != nil {
		return err
	}
	if err := db.Callback().Update().After("gorm:update").Register("apperr:unique_violation_update", translate); err != nil {
		return err
	}
	return db.Callback().Delete().After("gorm:delete").Register("apperr:unique_violation_delete", translate)
}

package repository

import (
	"errors"

	"payroll-backend/apperr"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// uniqueViolation is the SQLSTATE Postgres raises when a write breaks a unique index.
const uniqueViolation = "23505"

// TranslateUniqueViolation turns a Postgres unique-constraint failure into a Conflict, so a duplicate name answers 409 instead of a generic 500 the client cannot tell apart from a broken server.
func TranslateUniqueViolation(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return apperr.Conflict("A record with the same unique value already exists")
	}
	return err
}

// RegisterErrorTranslation makes every create and update on this connection report a unique violation as a Conflict.
// It is a connection-level callback rather than a call in each repository because the mapping is a property of Postgres, not of any one domain, and a repository that forgets it is a silent 500.
func RegisterErrorTranslation(db *gorm.DB) error {
	translate := func(tx *gorm.DB) {
		tx.Error = TranslateUniqueViolation(tx.Error)
	}
	if err := db.Callback().Create().After("gorm:create").Register("apperr:unique_violation_create", translate); err != nil {
		return err
	}
	return db.Callback().Update().After("gorm:update").Register("apperr:unique_violation_update", translate)
}

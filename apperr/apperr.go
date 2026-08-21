package apperr

import "errors"

// Kind classifies a failure so the HTTP layer can pick a status without knowing the domain.
type Kind int

const (
	// KindInternal is the zero value so an unclassified error can never leak as a 4xx.
	KindInternal Kind = iota
	KindValidation
	KindNotFound
	KindForbidden
	KindConflict
	KindUnauthorized
)

// Error pairs a client-safe message with its kind; driver or SQL text must never become one.
type Error struct {
	Kind    Kind
	Message string
}

func (e *Error) Error() string { return e.Message }

// Validation reports input the caller can correct and resend.
func Validation(message string) error {
	return &Error{Kind: KindValidation, Message: message}
}

// NotFound reports a missing row, or one deliberately hidden by scoping.
func NotFound(message string) error {
	return &Error{Kind: KindNotFound, Message: message}
}

// Forbidden reports a row the caller is authenticated for but not authorized to touch.
func Forbidden(message string) error {
	return &Error{Kind: KindForbidden, Message: message}
}

// Conflict reports a request that contradicts existing state.
func Conflict(message string) error {
	return &Error{Kind: KindConflict, Message: message}
}

// Unauthorized reports a missing or unusable authentication context.
func Unauthorized(message string) error {
	return &Error{Kind: KindUnauthorized, Message: message}
}

// Internal answers 500 with an authored message instead of the generic line.
func Internal(message string) error {
	return &Error{Kind: KindInternal, Message: message}
}

// KindOf classifies err, defaulting to KindInternal for anything not authored here.
func KindOf(err error) Kind {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Kind
	}
	return KindInternal
}

// Message returns the client-safe text, substituting a generic line for internal failures.
func Message(err error) string {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return "Request could not be completed"
}

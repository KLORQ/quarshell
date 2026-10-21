package domain

import (
	"errors"
	"fmt"
)

// Code classifies an application error independently of its transport.
type Code string

const (
	CodeInvalid          Code = "invalid"
	CodeNotFound         Code = "not_found"
	CodeConflict         Code = "conflict"
	CodeUnauthorized     Code = "unauthorized"
	CodePermissionDenied Code = "permission_denied"
	CodeUnsupported      Code = "unsupported"
	CodeUnavailable      Code = "unavailable"
	CodeCanceled         Code = "canceled"
	CodeDeadlineExceeded Code = "deadline_exceeded"
	CodeInternal         Code = "internal"
)

var (
	// ErrInvalid marks malformed or semantically invalid input.
	ErrInvalid = errors.New("invalid")
	// ErrNotFound marks a missing resource.
	ErrNotFound = errors.New("not found")
	// ErrConflict marks a generation, revision, or idempotency conflict.
	ErrConflict = errors.New("conflict")
	// ErrUnauthorized marks missing or invalid authentication.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrPermissionDenied marks an authenticated but forbidden action.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrUnsupported marks an unavailable capability or platform.
	ErrUnsupported = errors.New("unsupported")
	// ErrUnavailable marks a transient dependency failure.
	ErrUnavailable = errors.New("unavailable")
	// ErrCanceled marks context cancellation.
	ErrCanceled = errors.New("canceled")
	// ErrDeadlineExceeded marks an expired deadline.
	ErrDeadlineExceeded = errors.New("deadline exceeded")
	// ErrInternal marks an unexpected internal failure.
	ErrInternal = errors.New("internal")
)

// Error carries a stable code, operation, safe message, and optional cause.
// The cause is for local diagnostics and must not be sent to clients verbatim.
type Error struct {
	Code    Code
	Op      string
	Message string
	Err     error
}

// Error implements error.
func (e *Error) Error() string {
	switch {
	case e.Op != "" && e.Message != "":
		return e.Op + ": " + e.Message
	case e.Message != "":
		return e.Message
	case e.Op != "":
		return e.Op
	default:
		return string(e.Code)
	}
}

// Unwrap returns the underlying cause.
func (e *Error) Unwrap() error { return e.Err }

// Is supports errors.Is against both sentinel errors and matching code/op pairs.
func (e *Error) Is(target error) bool {
	if e.Err != nil && errors.Is(e.Err, target) {
		return true
	}
	switch target {
	case ErrInvalid:
		return e.Code == CodeInvalid
	case ErrNotFound:
		return e.Code == CodeNotFound
	case ErrConflict:
		return e.Code == CodeConflict
	case ErrUnauthorized:
		return e.Code == CodeUnauthorized
	case ErrPermissionDenied:
		return e.Code == CodePermissionDenied
	case ErrUnsupported:
		return e.Code == CodeUnsupported
	case ErrUnavailable:
		return e.Code == CodeUnavailable
	case ErrCanceled:
		return e.Code == CodeCanceled
	case ErrDeadlineExceeded:
		return e.Code == CodeDeadlineExceeded
	case ErrInternal:
		return e.Code == CodeInternal
	}
	var other *Error
	if errors.As(target, &other) {
		return other.Code == e.Code && (other.Op == "" || other.Op == e.Op)
	}
	return false
}

// Wrap attaches an operation and safe message to a classified error.
func Wrap(err error, op, message string) error {
	if err == nil {
		return nil
	}
	return &Error{Code: CodeOf(err), Op: op, Message: message, Err: err}
}

// New creates a classified error.
func New(code Code, op, message string) error {
	return &Error{Code: code, Op: op, Message: message}
}

// CodeOf returns the stable code for err, defaulting to CodeInternal.
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	var domainErr *Error
	if errors.As(err, &domainErr) {
		return domainErr.Code
	}
	switch {
	case errors.Is(err, ErrInvalid):
		return CodeInvalid
	case errors.Is(err, ErrNotFound):
		return CodeNotFound
	case errors.Is(err, ErrConflict):
		return CodeConflict
	case errors.Is(err, ErrUnauthorized):
		return CodeUnauthorized
	case errors.Is(err, ErrPermissionDenied):
		return CodePermissionDenied
	case errors.Is(err, ErrUnsupported):
		return CodeUnsupported
	case errors.Is(err, ErrUnavailable):
		return CodeUnavailable
	case errors.Is(err, ErrCanceled):
		return CodeCanceled
	case errors.Is(err, ErrDeadlineExceeded):
		return CodeDeadlineExceeded
	default:
		return CodeInternal
	}
}

// SafeMessage returns a message that may be logged or returned without secret
// material. It never includes the wrapped cause.
func SafeMessage(err error) string {
	if err == nil {
		return ""
	}
	var domainErr *Error
	if errors.As(err, &domainErr) {
		if domainErr.Message != "" {
			return domainErr.Message
		}
		return string(domainErr.Code)
	}
	return string(CodeOf(err))
}

// Invalidf builds an invalid-argument error.
func Invalidf(op, format string, args ...any) error {
	return &Error{Code: CodeInvalid, Op: op, Message: fmt.Sprintf(format, args...), Err: ErrInvalid}
}

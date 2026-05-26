package core

import (
	"errors"
	"fmt"
	"runtime"
)

// ErrorCode identifies the subsystem that produced the error.
type ErrorCode string

const (
	// ErrProvider indicates an error from the provider layer (API, network, auth).
	ErrProvider ErrorCode = "PROVIDER_ERROR"
	// ErrTool indicates a tool execution error.
	ErrTool ErrorCode = "TOOL_ERROR"
	// ErrSession indicates a session lifecycle error.
	ErrSession ErrorCode = "SESSION_ERROR"
	// ErrTimeout indicates a deadline/timeout error.
	ErrTimeout ErrorCode = "TIMEOUT"
	// ErrCancelled indicates the operation was cancelled.
	ErrCancelled ErrorCode = "CANCELLED"
	// ErrTurnInProgress indicates a turn is already running.
	ErrTurnInProgress ErrorCode = "TURN_IN_PROGRESS"
	// ErrConfig indicates a configuration error.
	ErrConfig ErrorCode = "CONFIG_ERROR"
	// ErrValidation indicates an input validation error.
	ErrValidation ErrorCode = "VALIDATION_ERROR"
	// ErrPermission indicates a permission denied error.
	ErrPermission ErrorCode = "PERMISSION_ERROR"
	// ErrNotFound indicates the requested resource was not found.
	ErrNotFound ErrorCode = "NOT_FOUND"
)

// TauError is the canonical error type for the tau framework.
type TauError struct {
	Code    ErrorCode
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *TauError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the wrapped cause error.
func (e *TauError) Unwrap() error {
	return e.Cause
}

// --- Kind-based error system ---

// ErrorKind classifies the nature of an error for retry/dispatch decisions.
type ErrorKind int

const (
	// KindTransient indicates a retryable error (429, timeout, connection).
	KindTransient ErrorKind = iota
	// KindPermanent indicates a non-retryable error (400, 401, validation).
	KindPermanent
	// KindUsage indicates a user error (wrong args, not found).
	KindUsage
)

// Error is a structured error with kind, operation, and file:line capture.
type Error struct {
	Kind ErrorKind
	Op   string // operation name (e.g., "openai.Stream", "loop.Prompt")
	Err  error  // underlying error
	File string // auto-captured source file
	Line int    // auto-captured source line
}

// Error implements the error interface with file:line context.
func (e *Error) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("%s:%d %s: %v", e.File, e.Line, e.Op, e.Err)
	}
	return e.Err.Error()
}

// Unwrap implements errors.Unwrap for Is/As compatibility.
func (e *Error) Unwrap() error { return e.Err }

// NewError creates a structured error with auto file:line capture.
func NewError(op string, kind ErrorKind, err error) *Error {
	_, file, line, _ := runtime.Caller(1)
	if idx := stringsLastIndex(file, "/"); idx >= 0 {
		file = file[idx+1:]
	}
	return &Error{Kind: kind, Op: op, Err: err, File: file, Line: line}
}

// Wrap wraps an existing error with operation context, preserving kind.
func Wrap(op string, err error) *Error {
	kind := KindOf(err)
	return NewError(op, kind, err)
}

// Cause returns the root cause by unwrapping all Error wrappers.
func Cause(err error) error {
	for {
		var e *Error
		if errors.As(err, &e) {
			err = e.Err
		} else {
			return err
		}
	}
}

// IsKind checks if any error in the chain has the given kind.
func IsKind(err error, kind ErrorKind) bool {
	for {
		var e *Error
		if errors.As(err, &e) {
			if e.Kind == kind {
				return true
			}
			err = e.Err
		} else {
			return false
		}
	}
}

// KindOf returns the ErrorKind of the first structured Error in the chain, or KindPermanent.
func KindOf(err error) ErrorKind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindPermanent
}

// Transient wraps an error as a transient (retryable) error.
func Transient(op string, err error) *Error {
	return NewError(op, KindTransient, err)
}

// Permanent wraps an error as a permanent (non-retryable) error.
func Permanent(op string, err error) *Error {
	return NewError(op, KindPermanent, err)
}

// UsageError wraps an error as a user/usage error.
func UsageError(op string, err error) *Error {
	return NewError(op, KindUsage, err)
}

func stringsLastIndex(s, sep string) int {
	for i := len(s) - len(sep); i >= 0; i-- {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}

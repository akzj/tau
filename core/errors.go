package core

import "fmt"

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

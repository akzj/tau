package core

import "fmt"

// ErrorCode identifies the subsystem that produced the error.
type ErrorCode string

const (
	ErrProvider  ErrorCode = "PROVIDER_ERROR"
	ErrTool      ErrorCode = "TOOL_ERROR"
	ErrSession   ErrorCode = "SESSION_ERROR"
	ErrTimeout       ErrorCode = "TIMEOUT"
	ErrCancelled     ErrorCode = "CANCELLED"
	ErrTurnInProgress ErrorCode = "TURN_IN_PROGRESS"
)

// TauError is the canonical error type for the tau framework.
type TauError struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *TauError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *TauError) Unwrap() error {
	return e.Cause
}

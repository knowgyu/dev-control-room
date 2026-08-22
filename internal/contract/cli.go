// Package contract defines the stable machine-facing CLI/HTTP envelope and
// error vocabulary. Presentation layers may format it differently, but they
// must not invent a second set of business errors.
package contract

import (
	"encoding/json"
	"errors"
	"fmt"
)

const EnvelopeSchema = "devroom/cli/v1"

type ErrorCode string

const (
	ErrorInternal        ErrorCode = "internal_error"
	ErrorInvalidInput    ErrorCode = "invalid_input"
	ErrorNotFound        ErrorCode = "not_found"
	ErrorConflict        ErrorCode = "conflict"
	ErrorForbidden       ErrorCode = "forbidden"
	ErrorPolicyDenied    ErrorCode = "policy_denied"
	ErrorCheckFailed     ErrorCode = "check_failed"
	ErrorExecutionFailed ErrorCode = "execution_failed"
	ErrorUnavailable     ErrorCode = "unavailable_capability"
)

type ExitCode int

const (
	ExitSuccess        ExitCode = 0
	ExitInternal       ExitCode = 1
	ExitInvalidInput   ExitCode = 2
	ExitCheckFailed    ExitCode = 3
	ExitPolicyDenied   ExitCode = 4
	ExitExecutionError ExitCode = 5
	ExitUnavailable    ExitCode = 6
	ExitNotFound       ExitCode = 7
	ExitConflict       ExitCode = 8
	ExitForbidden      ExitCode = 9
)

func (c ErrorCode) ExitCode() ExitCode {
	switch c {
	case ErrorInvalidInput:
		return ExitInvalidInput
	case ErrorCheckFailed:
		return ExitCheckFailed
	case ErrorPolicyDenied:
		return ExitPolicyDenied
	case ErrorExecutionFailed:
		return ExitExecutionError
	case ErrorUnavailable:
		return ExitUnavailable
	case ErrorNotFound:
		return ExitNotFound
	case ErrorConflict:
		return ExitConflict
	case ErrorForbidden:
		return ExitForbidden
	default:
		return ExitInternal
	}
}

type Error struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type Meta struct {
	RequestID string `json:"requestId,omitempty"`
}

type Envelope[T any] struct {
	Schema string `json:"schema"`
	OK     bool   `json:"ok"`
	Data   *T     `json:"data,omitempty"`
	Error  *Error `json:"error,omitempty"`
	Meta   Meta   `json:"meta,omitempty"`
}

func Success[T any](data T) Envelope[T] {
	return Envelope[T]{Schema: EnvelopeSchema, OK: true, Data: &data}
}

func Failure[T any](code ErrorCode, message string, details map[string]any) Envelope[T] {
	return Envelope[T]{
		Schema: EnvelopeSchema,
		OK:     false,
		Error:  &Error{Code: code, Message: message, Details: details},
	}
}

func FromError[T any](err error) Envelope[T] {
	if err == nil {
		return Success(*new(T))
	}
	classified := Classify(err)
	return Failure[T](classified.Code, classified.Message, classified.Details)
}

type CodedError struct {
	Code    ErrorCode
	Message string
	Details map[string]any
}

// ClassifiedError is the only error shape that presentation adapters should
// serialize. Unknown/internal errors deliberately lose their original
// message, which may contain filesystem paths, SQL, command output, or
// credentials.
type ClassifiedError struct {
	Code    ErrorCode
	Message string
	Details map[string]any
}

func Classify(err error) ClassifiedError {
	if err == nil {
		return ClassifiedError{Code: ErrorInternal, Message: "internal error"}
	}
	var coded CodedError
	if !errors.As(err, &coded) || !isPublicCode(coded.Code) {
		return ClassifiedError{Code: ErrorInternal, Message: "internal error"}
	}
	return ClassifiedError{
		Code:    coded.Code,
		Message: publicMessage(coded.Code, coded.Message),
		// Details are intentionally dropped at the common boundary until a
		// typed, separately reviewed safe-details contract exists.
		Details: nil,
	}
}

func isPublicCode(code ErrorCode) bool {
	switch code {
	case ErrorInvalidInput, ErrorNotFound, ErrorConflict, ErrorForbidden,
		ErrorPolicyDenied, ErrorCheckFailed, ErrorExecutionFailed, ErrorUnavailable:
		return true
	default:
		return false
	}
}

func publicMessage(code ErrorCode, message string) string {
	if message != "" {
		return message
	}
	switch code {
	case ErrorInvalidInput:
		return "invalid input"
	case ErrorNotFound:
		return "not found"
	case ErrorConflict:
		return "conflict"
	case ErrorForbidden:
		return "forbidden"
	case ErrorPolicyDenied:
		return "policy denied"
	case ErrorCheckFailed:
		return "check failed"
	case ErrorExecutionFailed:
		return "execution failed"
	case ErrorUnavailable:
		return "capability unavailable"
	default:
		return "internal error"
	}
}

func (e CodedError) Error() string {
	if e.Message == "" {
		return string(e.Code)
	}
	return e.Message
}

func InvalidInput(message string) error {
	return CodedError{Code: ErrorInvalidInput, Message: message}
}

func NotFound(message string) error {
	return CodedError{Code: ErrorNotFound, Message: message}
}

func Conflict(message string) error {
	return CodedError{Code: ErrorConflict, Message: message}
}

func Forbidden(message string) error {
	return CodedError{Code: ErrorForbidden, Message: message}
}

func (e Error) Error() string {
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func Marshal[T any](value Envelope[T]) ([]byte, error) {
	return json.Marshal(value)
}

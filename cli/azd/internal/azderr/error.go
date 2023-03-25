package azderr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// A structured error that wraps a standard error.
type Error struct {
	// The error that occurred.
	Err error
	// The operation being executed when the error occurred.
	Operation string
	// The error code of the operation.
	Code string
	// Details that can be serializable as JSON string.
	Details json.Marshaler

	// Whether the error has been reported.
	reported bool
}

// Displays the error message.
func (e *Error) Error() string {
	return e.Err.Error()
}

// Allows unwrapping of the inner error.
func (e *Error) Unwrap() error {
	return e.Err
}

// Service related details
type ServiceDetails struct {
	// A developer-friendly name for the service
	Name string
	// The HTTP method used for the service
	Method string
	// The HTTP status code returned by the service.
	StatusCode int
	// The correlation ID of the executed HTTP request.
	// This can either be set the client, or the returned ID from the service.
	CorrelationId string
	// The resource associated to the HTTP request
	Resource string
	// Error code. If unprovided, defaults from StatusCode.
	ErrorCode string
}

func NewArmServiceError(err error, s ServiceDetails) error {
	code := s.ErrorCode
	if code == "" {
		code = http.StatusText(s.StatusCode)
	}
	return &Error{
		Operation: fmt.Sprintf(
			"service.%s.%s.%s",
			s.Name,
			s.Resource,
			s.Method),
		Code:    code,
		Err:     err,
		Details: &s,
	}
}

// Client tooling related details
type ToolDetails struct {
	// The name of the tool
	Name string
	// Command path of the tool
	CmdPath []string
	// The exit code received after executing the tool
	ExitCode int
	// Flags set when executing the tool
	Flags []string
	// Error code.
	ErrorCode string
	// Operation looks like: client.<tool name>.<cmd path>
}

func NewToolError(err error, t ToolDetails) error {
	return &Error{
		Operation: fmt.Sprintf(
			"tool.%s.%s",
			t.Name,
			strings.ToLower(strings.Join(t.CmdPath, " "))),
		Code:    t.ErrorCode,
		Err:     err,
		Details: &t,
	}
}

package azderr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/azure/azure-dev/cli/azd/internal"
	"github.com/azure/azure-dev/cli/azd/internal/telemetry"
	"github.com/azure/azure-dev/cli/azd/internal/telemetry/fields"
)

// A structured error for reporting purposes.
type Error struct {
	// The operation being executed when the error occurred.
	Operation string
	// The error code of the operation
	Code string
	// Inner errors
	Inner []Error
	// Details
	Details interface{}
}

// Service related errors
type ServiceDetails struct {
	// A developer-friendly name for the service
	ServiceName string
	// The HTTP method used for the service
	Method string
	// The HTTP status code returned by the service.
	StatusCode int
	// The correlation ID of the executed HTTP request.
	// This can either be set the client, or the returned ID from the service.
	CorrelationId string
	// The resource associated to the HTTP request
	Resource string
	// Error code, if unprovided. Defaults from StatusCode.
	ErrorCode string
}

func NewArmServiceError(s ServiceDetails, err ...Error) Error {
	code := s.ErrorCode
	if code == "" {
		code = http.StatusText(s.StatusCode)
	}
	return Error{
		Operation: fmt.Sprintf(
			"service.%s.%s.%s",
			s.ServiceName,
			s.Resource,
			s.Method),
		Code:    code,
		Inner:   err,
		Details: s,
	}
}

// Client tooling related errors
type ToolDetails struct {
	// The name of the tool
	Name string
	// Command path of the tool
	Path []string
	// The exit code received after executing the tool
	ExitCode int
	// Flags set when executing the tool
	Flags []string
	// Error code.
	ErrorCode string
	// Operation looks like: client.<tool name>.<cmd path>
}

func NewToolError(t ToolDetails, err ...Error) Error {
	return Error{
		Operation: fmt.Sprintf(
			"tool.%s.%s",
			t.Name,
			strings.ToLower(strings.Join(t.Path, " "))),
		Code:    t.ErrorCode,
		Inner:   err,
		Details: t,
	}
}

func attachError(s telemetry.Span, e Error) {
	details, err := json.Marshal(e.Details)
	if err != nil && internal.IsDevVersion() {
		panic(err)
	}

	s.SetAttributes(
		fields.ErrOperation.String(e.Operation),
		fields.ErrCode.String(e.Code),
		fields.ErrDetails.String(string(details)),
	)
	s.SetAttributes(fields.ErrOperation.String(e.Operation))
}

var _ = attachError

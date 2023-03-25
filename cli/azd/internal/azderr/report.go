package azderr

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/azure/azure-dev/cli/azd/internal"
	"github.com/azure/azure-dev/cli/azd/internal/telemetry"
	"github.com/azure/azure-dev/cli/azd/internal/telemetry/fields"
	"go.opentelemetry.io/otel/codes"
)

var _ ErrReporter = &Error{}
var _ errReporter = &Error{}

// The conventional interface for representing a type that allows reporting of errors.
type ErrReporter interface {
	// Reports an error to a telemetry span. The error is expected to be reported only once.
	// If the error has already been reported, this method is a no-op.
	//
	// The reported error is transmitted with the span, when the span is transmitted.
	Report(s telemetry.Span)
}

// Internal interface for reporting inner errors.
type errReporter interface {
	// Returns true if the error has already been reported.
	isReported() bool
	// Reports the error with the following error frame as a JSON string.
	reportStr(frame int) string
}

// Reports the error to the telemetry span.
func (e *Error) Report(s telemetry.Span) {
	defer func() { e.reported = true }()
	if e.reported {
		return
	}
	details, err := json.Marshal(e.Details)
	if err != nil {
		LogOrPanic(fmt.Errorf("Report for %s: marshaling details: %w", e.Operation, err))
		return
	}

	s.SetAttributes(
		fields.ErrOperation.String(e.Operation),
		fields.ErrCode.String(e.Code),
		fields.ErrDetails.String(string(details)),
	)

	innerDetails := collectInner(e.Err)
	if len(innerDetails) > 0 {
		s.SetAttributes(fields.ErrInner.StringSlice(innerDetails))
	}

	s.SetStatus(codes.Error, e.Operation+"."+e.Code)
}

func (e *Error) reportStr(frame int) string {
	defer func() { e.reported = true }()
	if e.reported {
		return ""
	}
	details, err := json.Marshal(e.Details)
	if err != nil {
		LogOrPanic(fmt.Errorf("report for %s: marshaling details: %w", e.Operation, err))
		return ""
	}

	m := map[string]any{
		string(fields.ErrOperation): e.Operation,
		string(fields.ErrCode):      e.Code,
		string(fields.ErrFrame):     frame,
		string(fields.ErrDetails):   details,
	}

	reportStr, err := json.Marshal(m)
	if err != nil && internal.IsDev {
		LogOrPanic(fmt.Errorf("report for %s: marshaling report: %w", e.Operation, err))
		return ""
	}

	e.reported = true
	return string(reportStr)
}

// Logs the error if in non-dev mode, otherwise panics in dev mode.
func LogOrPanic(err error) {
	if internal.IsDev {
		log.Panicf("%v", err)
	} else {
		log.Printf("%v", err)
	}
}

// Collects all inner errors that satisfy errReporter and returns them as a slice of JSON strings.
func collectInner(err error) []string {
	if err == nil {
		return nil
	}

	var details []string
	frame := 0
	for {
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			err = x.Unwrap()
			if err == nil {
				return details
			}

			if errInfo, ok := err.(errReporter); ok {
				if errInfo.isReported() {
					// skip all inner
					return details
				}

				if report := errInfo.reportStr(frame); report != "" {
					details = append(details, report)
				}
			}
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if errInfo, ok := err.(errReporter); ok {
					if errInfo.isReported() {
						// skip all inner
						return details
					}

					if report := errInfo.reportStr(frame); report != "" {
						details = append(details, report)
					}
				}
			}
			return details
		default:
			return details
		}

		frame++
	}
}

func (e *Error) isReported() bool {
	return e.reported
}

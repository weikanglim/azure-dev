# Error Telemetry

## Background

Terminology:

- Event - an event is a discrete telemetry item that is captured in a logging system.
- Span - a span represents a unit of work of operation. It has StartTime, StopTime, and a ParentId for correlation to other units of work of operation.
- Metric - a highly aggregated type of numerical measurement.

[OpenTelemetry docs](https://opentelemetry.io/docs/concepts/signals/traces/#spans-in-opentelemetry) for full documentation.

## Emitting Errors

There are two widely used strategies at Microsoft when it comes to error telemetry at Microsoft:

1. Log error events. When an error occurs, log a discrete telemetry event that indicates the error event.
1. Attach error information to Spans (operations). It is assumed that when an error occurs, there's an active telemetry span. The error is attached to the telemetry span.

Approach 1, logging error events, are useful for signaling (a simple count of error suffices), while

Approach 2, attaching error to spans, are useful for troubleshooting.

There's largely been a shift to Approach 2 for general error reporting ever since the idea of distributed tracing/spans have been made easier and practical. Approach 1 can simply be done by generating Metrics.

For `azd`, we will choose approach 2.

## Error Schema

To fully understand the error schema, we should first examine the direction of where `azd` should head in, in the long run.

### Errors attached to individual spans

Ideally, we have error information attached to the closest Span. For example, I would imagine we would emit a Span for each service call. The `ServiceRequestSpan` (this is a made-up name just to refer to this type differently) would provide context about the service request. The error information simply contains details about the error.

In that world, an error to a `GET containerApp` request failing would look like:

```json
{
    "Name": "service.arm.microsoft.app.containerapps.get",
    "DurationMs": 4152.7521,
    "ResultCode": "service.notFound", // a useful human representation of what this error means for this operation. should be at the same granularity as what you'd expect for the operation
    "Success": false,
    "Measurements": {},
    "Properties": {
        "service.name": "arm",
        "service.status-code": 404,
        "service.correlationId": "c19d9005-b134-4b68-a3b3-0f1219ef79f4",
        "service.resource": "Microsoft.App/containerApps",
        "service.method": "GET",
        "service.url": "https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.App/containerApps/{containerAppName}?api-version=2022-03-01",
        "error.operation": "service.arm.microsoft.app.containerapps.get", // might not always match the operation on the Span. An inner-operation could have failed that lacks it's individual span representation
        "error.code": "notFound", // a useful human representation of what this error means for this operation
        "error.inner": [], // inner errors
        "error.details": {} // additional details about the error
    },
}
```

Notice how the `span` describes the information about the operation, and the `error` provides supplemental information around the failure. Also notice that we now have spans that record how long each operation took from `azd`'s client perspective.

I'd love to get us to this world where we do log individual spans for each service call, but there are some (solvable) challenges ahead with the telemetry upload pipeline [1789](https://github.com/Azure/azure-dev/issues/1789). We also need to talk to our DataX partners before we start implement tracing throughout the CLI. The below section introduces a V1 schema implementation that allows us to get useful telemetry without revisiting the telemetry upload pipeline, but is compatible with future efforts.

### V1 Error Schema: Attaching error to root `cmd` span

In `azd`, we emit a `cmd` span that represents the entire command invocation. We can attach the `error` in such a manner:

```json
{
    "Name": "cmd.provision",
    "DurationMs": 4152.7521,
    "ResultCode": "service.notFound", // a useful human representation of what this error means for this operation. should be at the same granularity as what you'd expect for the operation
    "Success": false,
    "Measurements": {},
    "Properties": {
        "error.operation": "service.arm.microsoft.app.containerapps.get", // might not always match the operation on the Span. An inner-operation could have failed that lacks it's individual span representation
        "error.code": "notFound", // a useful human representation of what this error means for this operation
        "error.inner": [], // inner errors
        "error.details": {
            "service.name": "arm",
            "service.status-code": 404,
            "service.correlationId": "c19d9005-b134-4b68-a3b3-0f1219ef79f4",
            "service.resource": "Microsoft.App/containerApps",
            "service.method": "GET",
            "service.url": "https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.App/containerApps/{containerAppName}?api-version=2022-03-01"
        } // additional details about the error
    },
}
```

Notice that the details about the Span have moved into the `error`. This makes sense logically as the `cmd` span shouldn't know about the details of the HTTP request, rather the `error` which is a `service.error` that contains HTTP details has the error.

This allows us to emit error events that are actionable, but also move the details back to the span in the near future.

## Error Implementation

Given that we want the error to be attached to a span, and we want structured errors, the following is true:

1. We need structured errors.
1. We need to separate error producer from error consumer, so that errors can be attached to any given span.

To meet number 1, we simply need to define a struct that captures the information we want.
To meet number 2, there are two approaches:

1. Error can be bubbled up using application-context fields. This currently exists in `telemetry`, with methods like `SetUsageAttributes`. When methods like these are called, the attributes are stored in a package-level that is safe for concurrent use.
1. Error can be bubbled up using the standard `error` chain.

The problem with approach 1, is that we want the error reporting to be highly flexible. While it is true that today, the error will only be reported close up to `main`, that will evolve.

Thus, we will choose approach 2.

### Error wrapping

To implement approach 2 effectively, we need to understand how we can have a structured error participate in the error wrapping chain.

An `error` in `golang` has these methods commonly defined:

- `Error() string` - Produces a string representation of the error. Required part of `error` interface.
- `Unwrap() error` - Returns the inner wrapped `error`. When you call `fmt.Errorf`, `go` returns a struct that is suitable for unwrapping. Weak interface.
- `Unwrap() error[]` - Returns the inner wrapped `error`s. This is new with go1.20. Weak interface.

Thus, we first define a `struct` that contains the fields we're interested about:

```golang
// A structured error that wraps a standard error.
type Error struct {
	// The operation being executed when the error occurred.
	Operation string
	// The error code of the operation.
	Code string
	// Details that can be serializable as JSON string.
	Details json.Marshaler
}
```

Then, we add the necessary logic to allow `Error` to participate in `error` wrapping chain:

```golang
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
}

// Displays the error message.
func (e *Error) Error() string {
	return e.Err.Error()
}

// Allows unwrapping of the inner error.
func (e *Error) Unwrap() error {
	return e.Err
}

// This is now valid syntax
var _ err error = &Error{}
```

This implementation delegates both `Error()` and `Unwrap()` to the underlying `err`. This means it's effectively invisible in the error chain when formatted.

Now, this allows the error producer to construct an `Error{}` struct that describes an operation, code, and details, alongside with the standard `error` to be displayed to the user.

To consume and report the error, the consumer simply needs to perform `errors.As(err, &Error)` and be able to pull out the error information, attaching it to a telemetry span if needed.

### 

Stack frame:

- root.go -> 
  - provision.go
    - service.go
      - client.go -> returns Error{}

## We need structured errors

```golang
type 
```

## Requirements

1. Need to capture error with all attributes. For service, this means correlation ID.
1. Should be easy to emit or report error. If `golang` reporting is used, I expect this to be idiomatic.

- One observation is that structured reporting allows us to also do formatting on the errors and sets us up for success. This would be huge.
- Sounds like we should go down this path. It'd be better than overwriting the error context.

1. Error can be set on `span` or be recorded to the `cmd` event

```json
{
    "error": {
        "operation": "service.arm.microsoft.app.containerapps.get",
        "code": "NotFound",
        "inner": {},
        "service.name": "arm",
        "service.status-code": 404,
        "service.correlationId": "c19d9005-b134-4b68-a3b3-0f1219ef79f4",
        "service.resource": "Microsoft.App/containerApps",
        "service.method": "GET",
        "service.url": "https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.App/containerApps/{containerAppName}?api-version=2022-03-01"
    },
}
```



```json
{
    "error": {
        "operation": "provision.bicep",
        "code": "DeploymentFailed",
        "infra.provider": "bicep",
        "inner": [
            {
                "operation": "service.arm.microsoft.app.containerapps.get",
                "code": "NotFound",
                "inner": {},
                "service.name": "arm",
                "service.status-code": 404,
                "service.correlationId": "c19d9005-b134-4b68-a3b3-0f1219ef79f4",
                "service.resource": "Microsoft.App/containerApps",
                "service.method": "GET",
                "service.url": "https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.App/containerApps/{containerAppName}?api-version=2022-03-01"
            }
        ]
    },
}
```

Or more like:


```json
{
    "Name": "cmd.provision",
    "DurationMs": 4152.7521,
    "ResultCode": "provision.bicep.deploymentFailed",
    "Success": false,
    "Measurements": {},
    "Properties": {
        "error.operation": "service.arm.microsoft.app.containerapps.get",
        "error.code": "NotFound",
        "error.inner": [
            {
                "error.operation": "service.arm.microsoft.app.containerapps.get",
                "error.code": "NotFound",
                "error.frame": 1,
                "error.details": {
                    "service.name": "arm",
                    "service.status-code": 404,
                    "service.correlationId": "c19d9005-b134-4b68-a3b3-0f1219ef79f4",
                    "service.resource": "Microsoft.App/containerApps",
                    "service.method": "GET",
                    "service.url": "https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.App/containerApps/{containerAppName}?api-version=2022-03-01"
                }
            }
        ],
        "error.details": {
            "service.name": "arm",
            "service.status-code": 404,
            "service.correlationId": "c19d9005-b134-4b68-a3b3-0f1219ef79f4",
            "service.resource": "Microsoft.App/containerApps",
            "service.method": "GET",
            "service.url": "https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.App/containerApps/{containerAppName}?api-version=2022-03-01"
        }
    },
}
```
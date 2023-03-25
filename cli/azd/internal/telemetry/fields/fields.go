// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

// Package fields provides definitions and functions related to the definition of telemetry fields.
package fields

import (
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

// Application-level fields. Guaranteed to be set and available for all events.
const (
	// Application name. Value is always "azd".
	ServiceNameKey = semconv.ServiceNameKey // service.name
	// Application version.
	ServiceVersionKey = semconv.ServiceVersionKey // service.version

	// The operating system type.
	OSTypeKey = semconv.OSTypeKey // os.type

	// The operating system version.
	//
	// Examples:
	//   - On Windows systems: The Windows version 10.x.x
	//   - On Unix-based systems: The release portion of uname. https://en.wikipedia.org/wiki/Uname#Examples
	//   - On MacOS: The MacOS version. For example: 12.5.1 for macOS Monterey
	OSVersionKey = semconv.OSVersionKey // os.version

	// The CPU architecture the host system is running on.
	HostArchKey = semconv.HostArchKey // host.arch

	// The version of the runtime of this process, as returned by the runtime without
	// modification.
	ProcessRuntimeVersionKey = semconv.ProcessRuntimeVersionKey // process.runtime.version

	// A unique ID associated to the machine the application is installed on.
	//
	// This shares implementation with VSCode's machineId and can match exactly on a given device, although there are no
	// guarantees.
	MachineIdKey = attribute.Key("machine.id")

	// An enumeration of possible environments that the application is running on.
	//
	// Example: Desktop, Azure Pipelines, Visual Studio.
	//
	// See EnvDesktop for complete set of values.
	ExecutionEnvironmentKey = attribute.Key("execution.environment")
)

// Context level fields. Availability depends on the command running.
const (
	// Object ID of the principal.
	ObjectIdKey = attribute.Key(contracts.UserAuthUserId) // user_AuthenticatedId
	// Tenant ID of the principal.
	TenantIdKey = attribute.Key("ad.tenant.id")
	// The type of account. See AccountTypeUser for all possible options.
	AccountTypeKey = attribute.Key("ad.account.type")
	// Currently selected Subscription ID.
	SubscriptionIdKey = attribute.Key("ad.subscription.id")
	// Currently selected Project Template ID.
	TemplateIdKey = attribute.Key("project.template.id")
)

// All possible enumerations of ExecutionEnvironmentKey
const (
	// Desktop environments

	EnvDesktop          = "Desktop"
	EnvVisualStudio     = "Visual Studio"
	EnvVisualStudioCode = "Visual Studio Code"

	// Hosted/Continuous Integration environments

	EnvUnknownCI          = "UnknownCI"
	EnvAzurePipelines     = "Azure Pipelines"
	EnvGitHubActions      = "GitHub Actions"
	EnvAppVeyor           = "AppVeyor"
	EnvBamboo             = "Bamboo"
	EnvBitBucketPipelines = "BitBucket Pipelines"
	EnvTravisCI           = "Travis CI"
	EnvCircleCI           = "Circle CI"
	EnvGitLabCI           = "GitLab CI"
	EnvJenkins            = "Jenkins"
	EnvAwsCodeBuild       = "AWS CodeBuild"
	EnvGoogleCloudBuild   = "Google Cloud Build"
	EnvTeamCity           = "TeamCity"
	EnvJetBrainsSpace     = "JetBrains Space"
	EnvCodespaces         = "GitHub Codespaces"
)

// All possible enumerations of AccountTypeKey
const (
	// A user.
	AccountTypeUser = "User"
	// A service principal, typically an application.
	AccountTypeServicePrincipal = "Service Principal"
)

// The value used for ServiceNameKey
const ServiceNameAzd = "azd"

// Additional fields of events.AccountSubscriptionsListEvent
const (
	// Number of tenants found
	AccountSubscriptionsListTenantsFound = attribute.Key("tenants.found")
	// Number of tenants where listing of subscriptions failed
	AccountSubscriptionsListTenantsFailed = attribute.Key("tenants.failed")
)

// Error reporting fields
const (
	ErrOperation = attribute.Key("error.operation")
	ErrCode      = attribute.Key("error.code")
	ErrInner     = attribute.Key("error.inner")
	ErrFrame     = attribute.Key("error.frame")
	ErrDetails   = attribute.Key("error.details")
)

// Service related fields
const (
	// Service name
	ServiceName = attribute.Key("service.name")
	// Service resource being requested
	ServiceResource = attribute.Key("service.resource")
	// Correlation ID of the service request
	ServiceCorrelationId = attribute.Key("service.correlationId")
	// Method of the service request
	ServiceMethod = attribute.Key("service.method")
	// Status code of the service request
	ServiceStatusCode = attribute.Key("service.statusCode")
	// Endpoint of the service request
	ServiceEndpoint = attribute.Key("service.endpoint")
)

// Tool related fields
const (
	// Name of the tool
	ToolName = attribute.Key("tool.name")
	// Version of the tool
	ToolVersion = attribute.Key("tool.version")
	// Command executed for a given tool.
	ToolCommand = attribute.Key("tool.command")
	// Exit code for a given command
	ToolExitCode = attribute.Key("tool.exitCode")
	// Flags set when a command is executed for a given tool.
	ToolFlags = attribute.Key("tool.flags")
)

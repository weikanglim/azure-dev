package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/azure/azure-dev/cli/azd/cmd/actions"
	"github.com/azure/azure-dev/cli/azd/cmd/middleware"
	"github.com/azure/azure-dev/cli/azd/pkg/exec"
	"github.com/azure/azure-dev/cli/azd/pkg/input"
	"github.com/azure/azure-dev/cli/azd/pkg/ioc"
	"github.com/azure/azure-dev/cli/azd/pkg/output"
	"github.com/azure/azure-dev/cli/azd/pkg/output/ux"
	"github.com/azure/azure-dev/cli/azd/pkg/tools"
	"github.com/azure/azure-dev/cli/azd/pkg/tools/azcli"
	"github.com/spf13/cobra"
)

// CobraBuilder manages the construction of the cobra command tree from nested ActionDescriptors
type CobraBuilder struct {
	container *ioc.NestedContainer
	runner    *middleware.MiddlewareRunner
}

// Creates a new instance of the Cobra builder
func NewCobraBuilder(container *ioc.NestedContainer) *CobraBuilder {
	return &CobraBuilder{
		container: container,
		runner:    middleware.NewMiddlewareRunner(container),
	}
}

// Builds a cobra Command for the specified action descriptor
func (cb *CobraBuilder) BuildCommand(descriptor *actions.ActionDescriptor) (*cobra.Command, error) {
	cmd := descriptor.Options.Command
	if cmd.Use == "" {
		cmd.Use = descriptor.Name
	}

	// Build the full command tree
	for _, childDescriptor := range descriptor.Children() {
		childCmd, err := cb.BuildCommand(childDescriptor)
		if err != nil {
			return nil, err
		}

		cmd.AddCommand(childCmd)
	}

	// Bind root command after command tree has been established
	// This ensures the command path is ready and consistent across all nested commands
	if descriptor.Parent() == nil {
		if err := cb.bindCommand(cmd, descriptor); err != nil {
			return nil, err
		}
	}

	// Configure action resolver for all commands
	if err := cb.configureActionResolver(cmd, descriptor); err != nil {
		return nil, err
	}

	return cmd, nil
}

func handleDocsFlag(
	ctx context.Context, cmd *cobra.Command, container *ioc.NestedContainer, docsUrl string) (bool, error) {
	// Handle --docs flags for all commands
	// Each command can use the description.options.DocumentationUrl to set what url to browse.
	// By default, azd will use documentationHostName
	if openDocs, _ := cmd.Flags().GetBool("docs"); openDocs {
		var console input.Console
		err := container.Resolve(&console)
		if err != nil {
			return false, err
		}
		commandDocsUrl := docsUrl
		if commandDocsUrl == "" {
			commandDocsUrl = documentationHostName
		}
		OpenWithDefaultBrowser(ctx, console, commandDocsUrl)
		return true, nil
	}
	return false, nil
}

func (cb *CobraBuilder) defaultCommandNoAction(
	ctx context.Context, cmd *cobra.Command, descriptor *actions.ActionDescriptor) error {

	flagHandled, err := handleDocsFlag(ctx, cmd, cb.container, descriptor.Options.DocumentationUrl)
	if err != nil {
		return err
	}
	if flagHandled {
		return nil
	}

	// when no --docs arg, display command's help
	return cmd.Help()
}

// Configures the cobra command 'RunE' function to running the composed middleware and action for the
// current action descriptor
func (cb *CobraBuilder) configureActionResolver(cmd *cobra.Command, descriptor *actions.ActionDescriptor) error {
	// Dev Error: Both action resolver and RunE have been defined
	if descriptor.Options.ActionResolver != nil && cmd.RunE != nil {
		return fmt.Errorf(
			//nolint:lll
			"action descriptor for '%s' must be configured with either an ActionResolver or a Cobra RunE command but NOT both",
			cmd.CommandPath(),
		)
	}

	// Only bind command to action if an action resolver had been defined
	// and when a RunE hasn't already been set
	if cmd.RunE != nil {
		return nil
	}

	// Neither cmd.RunE or descriptor.Options.ActionResolver set.
	// Set default behavior to either --docs or help
	if descriptor.Options.ActionResolver == nil {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ctx = tools.WithInstalledCheckCache(ctx)
			return cb.defaultCommandNoAction(ctx, cmd, descriptor)
		}
		return nil
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		ctx = tools.WithInstalledCheckCache(ctx)

		flagHandled, err := handleDocsFlag(ctx, cmd, cb.container, descriptor.Options.DocumentationUrl)
		if err != nil {
			return nil
		}
		if flagHandled {
			return nil
		}

		// Registers the following to enable injection into actions that require them
		ioc.RegisterInstance(cb.container, cb.runner)
		ioc.RegisterInstance(cb.container, middleware.MiddlewareContext(cb.runner))
		ioc.RegisterInstance(cb.container, ctx)
		ioc.RegisterInstance(cb.container, cmd)
		ioc.RegisterInstance(cb.container, args)

		if err := cb.registerMiddleware(descriptor); err != nil {
			return err
		}

		actionName := createActionName(cmd)
		var action actions.Action
		if err := cb.container.ResolveNamed(actionName, &action); err != nil {
			if errors.Is(err, ioc.ErrResolveInstance) {
				return fmt.Errorf(
					//nolint:lll
					"failed resolving action '%s'. Ensure the ActionResolver is a valid go function that returns an `actions.Action` interface, %w",
					actionName,
					err,
				)
			}

			return err
		}

		runOptions := &middleware.Options{
			Name:        cmd.Name(),
			CommandPath: cmd.CommandPath(),
			Aliases:     cmd.Aliases,
			Flags:       cmd.Flags(),
			Args:        args,
		}

		// Run the middleware chain with action
		log.Printf("Resolved action '%s'\n", actionName)
		actionResult, err := cb.runner.RunAction(ctx, runOptions, action)

		// At this point, we know that there might be an error, so we can silence cobra from showing it after us.
		cmd.SilenceErrors = true

		// TODO: Consider refactoring to move the UX writing to a middleware
		invokeErr := cb.container.Invoke(func(console input.Console) {
			var displayResult *ux.ActionResult
			if actionResult != nil && actionResult.Message != nil {
				displayResult = &ux.ActionResult{
					SuccessMessage: actionResult.Message.Header,
					FollowUp:       actionResult.Message.FollowUp,
				}
			} else if err != nil {
				displayResult = &ux.ActionResult{
					Err: err,
				}
			}

			if displayResult != nil {
				console.MessageUxItem(ctx, displayResult)
			}

			if err != nil {
				var respErr *azcore.ResponseError
				var azureErr *azcli.AzureDeploymentError
				var toolExitErr *exec.ExitError

				// We only want to show trace ID for server-related errors,
				// where we have full server logs to troubleshoot from.
				//
				// For client errors, we don't want to show the trace ID, as it is not useful to the user currently.
				if errors.As(err, &respErr) ||
					errors.As(err, &azureErr) ||
					(errors.As(err, &toolExitErr) && toolExitErr.Cmd == "terraform") {
					if actionResult != nil && actionResult.TraceID != "" {
						console.Message(
							ctx,
							output.WithErrorFormat(fmt.Sprintf("TraceID: %s", actionResult.TraceID)))
					}
				}
			}
		})

		if invokeErr != nil {
			return invokeErr
		}

		return err
	}

	return nil
}

// Binds the intersection of cobra command options and action descriptor options
func (cb *CobraBuilder) bindCommand(cmd *cobra.Command, descriptor *actions.ActionDescriptor) error {
	actionName := createActionName(cmd)

	// Automatically adds a consistent help flag
	cmd.Flags().BoolP("help", "h", false, fmt.Sprintf("Gets help for %s.", cmd.Name()))

	// Consistently registers output formats for the descriptor
	if len(descriptor.Options.OutputFormats) > 0 {
		output.AddOutputParam(cmd, descriptor.Options.OutputFormats, descriptor.Options.DefaultFormat)
	}

	// Create, register and bind flags when required
	if descriptor.Options.FlagsResolver != nil {
		ioc.RegisterInstance(cb.container, cmd)

		// The flags resolver is constructed and bound to the cobra command via dependency injection
		// This allows flags to be options and support any set of required dependencies
		if err := cb.container.RegisterSingletonAndInvoke(descriptor.Options.FlagsResolver); err != nil {
			return fmt.Errorf(
				//nolint:lll
				"failed registering FlagsResolver for action '%s'. Ensure the resolver is a valid go function and resolves without error. %w",
				actionName,
				err,
			)
		}
	}

	// Registers and bind action resolves when required
	// Action resolvers are essential go functions that create the instance of the required actions.Action
	// These functions are typically the constructor function for the action. ex) newDeployAction(...)
	// Action resolvers can take any number of dependencies and instantiated via the IoC container
	if descriptor.Options.ActionResolver != nil {
		if err := cb.container.RegisterNamedSingleton(actionName, descriptor.Options.ActionResolver); err != nil {
			return fmt.Errorf(
				//nolint:lll
				"failed registering ActionResolver for action'%s'. Ensure the resolver is a valid go function and resolves without error. %w",
				actionName,
				err,
			)
		}
	}

	// Bind flag completions
	// Since flags are lazily loaded we need to wait until after command flags are wired up before
	// any flag completion functions are registered
	for flag, completionFn := range descriptor.FlagCompletions() {
		if err := cmd.RegisterFlagCompletionFunc(flag, completionFn); err != nil {
			return fmt.Errorf("failed registering flag completion function for '%s', %w", flag, err)
		}
	}

	// Bind the child commands for the current descriptor
	for _, childDescriptor := range descriptor.Children() {
		childCmd := childDescriptor.Options.Command
		if err := cb.bindCommand(childCmd, childDescriptor); err != nil {
			return err
		}
	}

	if descriptor.Options.GroupingOptions.RootLevelHelp != actions.CmdGroupNone {
		if cmd.Annotations == nil {
			cmd.Annotations = make(map[string]string)
		}
		actions.SetGroupCommandAnnotation(cmd, descriptor.Options.GroupingOptions.RootLevelHelp)
	}

	// `generateCmdHelp` sets a default help section when `descriptor.Options.HelpOptions` is nil.
	// This call ensures all commands gets the same help formatting.
	cmd.SetHelpTemplate(generateCmdHelp(cmd, generateCmdHelpOptions{
		Description: cmdHelpGenerator(descriptor.Options.HelpOptions.Description),
		Usage:       cmdHelpGenerator(descriptor.Options.HelpOptions.Usage),
		Commands:    cmdHelpGenerator(descriptor.Options.HelpOptions.Commands),
		Flags:       cmdHelpGenerator(descriptor.Options.HelpOptions.Flags),
		Footer:      cmdHelpGenerator(descriptor.Options.HelpOptions.Footer),
	}))

	return nil
}

// Registers all middleware components for the current command and any parent descriptors
// Middleware components are insure to run in the order that they were registered from the
// root registration, down through action groups and ultimately individual actions
func (cb *CobraBuilder) registerMiddleware(descriptor *actions.ActionDescriptor) error {
	chain := []*actions.MiddlewareRegistration{}
	current := descriptor

	// Recursively loop through any action describer and their parents
	for {
		middleware := current.Middleware()

		for i := len(middleware) - 1; i > -1; i-- {
			registration := middleware[i]

			// Only use the middleware when the predicate resolves truthy or if not defined
			// Registration predicates are useful for when you want to selectively want to
			// register a middleware based on the descriptor options
			// Ex) Telemetry middleware registered for all actions except 'version'
			if registration.Predicate == nil || registration.Predicate(descriptor) {
				chain = append(chain, middleware[i])
			}
		}

		if current.Parent() == nil {
			break
		}

		current = current.Parent()
	}

	// Register middleware in reverse order so middleware registered
	// higher up the command structure are resolved before lower registrations
	for i := len(chain) - 1; i > -1; i-- {
		registration := chain[i]
		if err := cb.runner.Use(registration.Name, registration.Resolver); err != nil {
			return err
		}
	}

	return nil
}

// Composes a consistent action name for the specified cobra command
// ex) azd config list becomes 'azd-config-list-action'
func createActionName(cmd *cobra.Command) string {
	actionName := cmd.CommandPath()
	actionName = strings.TrimSpace(actionName)
	actionName = strings.ReplaceAll(actionName, " ", "-")
	actionName = fmt.Sprintf("%s-action", actionName)

	return strings.ToLower(actionName)
}

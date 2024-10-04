package aery

import (
	"context"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/azure/azure-dev/cli/azd/internal/aery"
	"github.com/azure/azure-dev/cli/azd/pkg/account"
	"github.com/azure/azure-dev/cli/azd/pkg/cloud"
	"github.com/azure/azure-dev/cli/azd/pkg/environment"
	"github.com/azure/azure-dev/cli/azd/pkg/infra/provisioning"
	"github.com/azure/azure-dev/cli/azd/pkg/input"
	"github.com/azure/azure-dev/cli/azd/pkg/prompt"
)

// NewAeryProvider creates a new instance of an aery provider
func NewAeryProvider(
	envManager environment.Manager,
	env *environment.Environment,
	console input.Console,
	prompters prompt.Prompter,
	cloud *cloud.Cloud,
	armClientOptions *arm.ClientOptions,
	account account.SubscriptionCredentialProvider,
) provisioning.Provider {
	return &aeryProvider{
		envManager:       envManager,
		env:              env,
		console:          console,
		portalUrlBase:    cloud.PortalUrlBase,
		prompters:        prompters,
		account:          account,
		armClientOptions: armClientOptions,
	}
}

type aeryProvider struct {
	root string

	envManager       environment.Manager
	env              *environment.Environment
	console          input.Console
	prompters        prompt.Prompter
	account          account.SubscriptionCredentialProvider
	armClientOptions *arm.ClientOptions

	portalUrlBase string
}

// Initialize implements provisioning.Provider.
func (a *aeryProvider) Initialize(ctx context.Context, projectPath string, options provisioning.Options) error {
	a.root = options.Path
	if a.root == "" {
		a.root = filepath.Join(projectPath, "infra")
	}

	return provisioning.EnsureSubscriptionAndLocation(
		ctx,
		a.envManager,
		a.env,
		a.prompters,
		nil,
	)
}

// Deploy implements provisioning.Provider.
func (a *aeryProvider) Deploy(ctx context.Context) (*provisioning.DeployResult, error) {
	subscriptionId := a.env.GetSubscriptionId()
	cred, err := a.account.CredentialForSubscription(ctx, subscriptionId)
	if err != nil {
		return nil, err
	}

	err = aery.Apply(
		ctx,
		filepath.Join(a.root, "ai.yaml"),
		subscriptionId,
		"rg-weilim-ai-01",
		cred,
		aery.ApplyOptions{
			ClientOptions: a.armClientOptions,
		})
	if err != nil {
		return nil, err
	}

	return &provisioning.DeployResult{
		Deployment: &provisioning.Deployment{
			Parameters: map[string]provisioning.InputParameter{},
			Outputs:    map[string]provisioning.OutputParameter{},
		},
	}, nil
}

// Destroy implements provisioning.Provider.
func (a *aeryProvider) Destroy(ctx context.Context, options provisioning.DestroyOptions) (*provisioning.DestroyResult, error) {
	panic("unimplemented")
}

// EnsureEnv implements provisioning.Provider.
func (a *aeryProvider) EnsureEnv(ctx context.Context) error {
	panic("unimplemented")
}

// Name implements provisioning.Provider.
func (a *aeryProvider) Name() string {
	return "aery"
}

// Preview implements provisioning.Provider.
func (a *aeryProvider) Preview(ctx context.Context) (*provisioning.DeployPreviewResult, error) {
	panic("unimplemented")
}

// State implements provisioning.Provider.
func (a *aeryProvider) State(ctx context.Context, options *provisioning.StateOptions) (*provisioning.StateResult, error) {
	panic("unimplemented")
}

var _ provisioning.Provider = &aeryProvider{}

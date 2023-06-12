// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package project

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/azure/azure-dev/cli/azd/pkg/azure"
	"github.com/azure/azure-dev/cli/azd/pkg/environment"
	"github.com/azure/azure-dev/cli/azd/pkg/infra"
	"github.com/azure/azure-dev/cli/azd/pkg/tools"
	"github.com/azure/azure-dev/cli/azd/pkg/tools/azcli"
)

type appServiceTarget struct {
	env *environment.Environment
	cli azcli.AzCli
}

// NewAppServiceTarget creates a new instance of the AppServiceTarget
func NewAppServiceTarget(
	env *environment.Environment,
	azCli azcli.AzCli,
) ServiceTarget {

	return &appServiceTarget{
		env: env,
		cli: azCli,
	}
}

// Gets the required external tools
func (st *appServiceTarget) RequiredExternalTools(context.Context) []tools.ExternalTool {
	return []tools.ExternalTool{}
}

// Initializes the AppService target
func (st *appServiceTarget) Initialize(ctx context.Context, serviceConfig *ServiceConfig) error {
	return nil
}

// Prepares a zip archive from the specified build output
func (st *appServiceTarget) Package(
	ctx context.Context,
	serviceConfig *ServiceConfig,
	packageOutput *ServicePackageResult,
	showProgress ShowProgress,
) (ServicePackageResult, error) {
	showProgress("Compressing deployment artifacts")
	zipFilePath, err := createDeployableZip(serviceConfig.Name, packageOutput.PackagePath)
	if err != nil {
		return ServicePackageResult{}, err
	}

	return ServicePackageResult{
		Build:       packageOutput.Build,
		PackagePath: zipFilePath,
	}, nil
}

// Deploys the prepared zip archive using Zip deploy to the Azure App Service resource
func (st *appServiceTarget) Deploy(
	ctx context.Context,
	serviceConfig *ServiceConfig,
	packageOutput *ServicePackageResult,
	targetResource *environment.TargetResource,
	showProgress ShowProgress,
) (ServiceDeployResult, error) {
	if err := st.validateTargetResource(ctx, serviceConfig, targetResource); err != nil {
		return ServiceDeployResult{}, fmt.Errorf("validating target resource: %w", err)
	}

	zipFile, err := os.Open(packageOutput.PackagePath)
	if err != nil {
		return ServiceDeployResult{}, fmt.Errorf("failed reading deployment zip file: %w", err)
	}

	defer os.Remove(packageOutput.PackagePath)
	defer zipFile.Close()

	showProgress("Uploading deployment package")
	res, err := st.cli.DeployAppServiceZip(
		ctx,
		targetResource.SubscriptionId(),
		targetResource.ResourceGroupName(),
		targetResource.ResourceName(),
		zipFile,
	)
	if err != nil {
		return ServiceDeployResult{}, fmt.Errorf("deploying service %s: %w", serviceConfig.Name, err)
	}

	showProgress("Fetching endpoints for app service")
	endpoints, err := st.Endpoints(ctx, serviceConfig, targetResource)
	if err != nil {
		return ServiceDeployResult{}, err
	}

	sdr := ServiceDeployResult{
		TargetResourceId: azure.WebsiteRID(
			targetResource.SubscriptionId(),
			targetResource.ResourceGroupName(),
			targetResource.ResourceName(),
		),
		Kind:      AppServiceTarget,
		Endpoints: endpoints,
		Details:   jsonStringOrUnmarshaled(*res),
	}
	sdr.Package = packageOutput

	return sdr, nil
}

// Gets the exposed endpoints for the App Service
func (st *appServiceTarget) Endpoints(
	ctx context.Context,
	serviceConfig *ServiceConfig,
	targetResource *environment.TargetResource,
) ([]string, error) {
	appServiceProperties, err := st.cli.GetAppServiceProperties(
		ctx,
		targetResource.SubscriptionId(),
		targetResource.ResourceGroupName(),
		targetResource.ResourceName(),
	)
	if err != nil {
		return nil, fmt.Errorf("fetching service properties: %w", err)
	}

	endpoints := make([]string, len(appServiceProperties.HostNames))
	for idx, hostName := range appServiceProperties.HostNames {
		endpoints[idx] = fmt.Sprintf("https://%s/", hostName)
	}

	return endpoints, nil
}

func (st *appServiceTarget) validateTargetResource(
	ctx context.Context,
	serviceConfig *ServiceConfig,
	targetResource *environment.TargetResource,
) error {
	if !strings.EqualFold(targetResource.ResourceType(), string(infra.AzureResourceTypeWebSite)) {
		return resourceTypeMismatchError(
			targetResource.ResourceName(),
			targetResource.ResourceType(),
			infra.AzureResourceTypeWebSite,
		)
	}

	return nil
}

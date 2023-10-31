// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/azure/azure-dev/cli/azd/pkg/async"
	"github.com/azure/azure-dev/cli/azd/pkg/azure"
	"github.com/azure/azure-dev/cli/azd/pkg/containerapps"
	"github.com/azure/azure-dev/cli/azd/pkg/environment"
	"github.com/azure/azure-dev/cli/azd/pkg/infra"
	"github.com/azure/azure-dev/cli/azd/pkg/tools"
)

const (
	cManifestRoot         = "manifest"
	cManifestTemplateFile = "containerApp.tmpl.yaml"
	cManifestFile         = "containerApp.yaml"
)

type containerAppTarget struct {
	env                 *environment.Environment
	envManager          environment.Manager
	containerHelper     *ContainerHelper
	containerAppService containerapps.ContainerAppService
	resourceManager     ResourceManager
}

// NewContainerAppTarget creates the container app service target.
func NewContainerAppTarget(
	env *environment.Environment,
	envManager environment.Manager,
	containerHelper *ContainerHelper,
	containerAppService containerapps.ContainerAppService,
	resourceManager ResourceManager,
) ServiceTarget {
	return &containerAppTarget{
		env:                 env,
		envManager:          envManager,
		containerHelper:     containerHelper,
		containerAppService: containerAppService,
		resourceManager:     resourceManager,
	}
}

// Gets the required external tools
func (at *containerAppTarget) RequiredExternalTools(ctx context.Context) []tools.ExternalTool {
	return at.containerHelper.RequiredExternalTools(ctx)
}

// Initializes the Container App target
func (at *containerAppTarget) Initialize(ctx context.Context, serviceConfig *ServiceConfig) error {
	if err := at.addPreProvisionChecks(ctx, serviceConfig); err != nil {
		return fmt.Errorf("initializing container app target: %w", err)
	}

	return nil
}

// Prepares and tags the container image from the build output based on the specified service configuration
func (at *containerAppTarget) Package(
	ctx context.Context,
	serviceConfig *ServiceConfig,
	packageOutput *ServicePackageResult,
) *async.TaskWithProgress[*ServicePackageResult, ServiceProgress] {
	return async.RunTaskWithProgress(
		func(task *async.TaskContextWithProgress[*ServicePackageResult, ServiceProgress]) {
			task.SetResult(packageOutput)
		},
	)
}

// Deploys service container images to ACR and provisions the container app service.
// The target resource can be partially filled with only ResourceGroupName, since container apps
// can be provisioned during deployment.
func (at *containerAppTarget) Deploy(
	ctx context.Context,
	serviceConfig *ServiceConfig,
	packageOutput *ServicePackageResult,
	targetResource *environment.TargetResource,
) *async.TaskWithProgress[*ServiceDeployResult, ServiceProgress] {
	return async.RunTaskWithProgress(
		func(task *async.TaskContextWithProgress[*ServiceDeployResult, ServiceProgress]) {
			manifestRoot := filepath.Join(serviceConfig.Path(), "manifest")
			manifestDeployment := false
			if targetResource.ResourceName() == "" {
				manifestDeployment, err := manifestExists(manifestRoot)
				if err != nil {
					task.SetError(err)
					return
				}

				if !manifestDeployment {
					// manifest doesn't exist. The resource should have been provisioned
					// Try refetching the resource, and if not, provide the error.
					res, err := at.resourceManager.GetServiceResource(
						ctx,
						targetResource.SubscriptionId(),
						targetResource.ResourceGroupName(),
						serviceConfig,
						"provision")
					if err != nil {
						task.SetError(err)
						return
					}

					targetResource = environment.NewTargetResource(
						targetResource.SubscriptionId(),
						targetResource.ResourceGroupName(),
						res.Name,
						res.Type,
					)
				} else {
					containerEnvName, err := getContainerAppEnvName(at.env, serviceConfig)
					if err != nil {
						task.SetError(err)
						return
					}
				}
			}

			if err := at.validateTargetResource(ctx, serviceConfig, targetResource); err != nil {
				task.SetError(fmt.Errorf("validating target resource: %w", err))
				return
			}

			// Login, tag & push container image to ACR
			containerDeployTask := at.containerHelper.Deploy(
				ctx, serviceConfig, packageOutput, targetResource.SubscriptionId())
			syncProgress(task, containerDeployTask.Progress())

			_, err := containerDeployTask.Await()
			if err != nil {
				task.SetError(err)
				return
			}

			imageName := at.env.GetServiceProperty(serviceConfig.Name, "IMAGE_NAME")
			task.SetProgress(NewServiceProgress("Updating container app revision"))

			err = at.containerAppService.AddRevision(
				ctx,
				targetResource.SubscriptionId(),
				targetResource.ResourceGroupName(),
				targetResource.ResourceName(),
				imageName,
			)
			if err != nil {
				task.SetError(fmt.Errorf("updating container app service: %w", err))
				return
			}

			task.SetProgress(NewServiceProgress("Fetching endpoints for container app service"))
			endpoints, err := at.Endpoints(ctx, serviceConfig, targetResource)
			if err != nil {
				task.SetError(err)
				return
			}

			task.SetResult(&ServiceDeployResult{
				Package: packageOutput,
				TargetResourceId: azure.ContainerAppRID(
					targetResource.SubscriptionId(),
					targetResource.ResourceGroupName(),
					targetResource.ResourceName(),
				),
				Kind:      ContainerAppTarget,
				Endpoints: endpoints,
			})
		},
	)
}

// Gets endpoint for the container app service
func (at *containerAppTarget) Endpoints(
	ctx context.Context,
	serviceConfig *ServiceConfig,
	targetResource *environment.TargetResource,
) ([]string, error) {
	if ingressConfig, err := at.containerAppService.GetIngressConfiguration(
		ctx,
		targetResource.SubscriptionId(),
		targetResource.ResourceGroupName(),
		targetResource.ResourceName(),
	); err != nil {
		return nil, fmt.Errorf("fetching service properties: %w", err)
	} else {
		endpoints := make([]string, len(ingressConfig.HostNames))
		for idx, hostName := range ingressConfig.HostNames {
			endpoints[idx] = fmt.Sprintf("https://%s/", hostName)
		}

		return endpoints, nil
	}
}

func (at *containerAppTarget) validateTargetResource(
	ctx context.Context,
	serviceConfig *ServiceConfig,
	targetResource *environment.TargetResource,
) error {
	if targetResource.ResourceGroupName() == "" {
		return fmt.Errorf("missing resource group name: %s", targetResource.ResourceGroupName())
	}

	if targetResource.ResourceType() != "" {
		if err := checkResourceType(targetResource, infra.AzureResourceTypeContainerApp); err != nil {
			return err
		}
	}

	return nil
}

func (at *containerAppTarget) addPreProvisionChecks(ctx context.Context, serviceConfig *ServiceConfig) error {
	// Attempt to retrieve the target resource for the current service
	// This allows the resource deployment to detect whether or not to pull existing container image during
	// provision operation to avoid resetting the container app back to a default image
	return serviceConfig.Project.AddHandler("preprovision", func(ctx context.Context, args ProjectLifecycleEventArgs) error {
		exists := false

		// Check if the target resource already exists
		targetResource, err := at.resourceManager.GetTargetResource(ctx, at.env.GetSubscriptionId(), serviceConfig)
		if targetResource != nil && err == nil {
			exists = true
		}

		at.env.SetServiceProperty(serviceConfig.Name, "RESOURCE_EXISTS", strconv.FormatBool(exists))
		return at.envManager.Save(ctx, at.env)
	})
}

func getContainerAppEnvName(env *environment.Environment, serviceConfig *ServiceConfig) (string, error) {
	containerEnvName := env.GetServiceProperty(serviceConfig.Name, "CONTAINER_ENVIRONMENT_NAME")
	if containerEnvName == "" {
		containerEnvName = env.Getenv("AZURE_CONTAINER_APPS_ENVIRONMENT_ID")
		if containerEnvName == "" {
			return "", fmt.Errorf(
				"could not determine container app environment for service %s, "+
					"have you set AZURE_CONTAINER_ENVIRONMENT_NAME or "+
					"SERVICE_%s_CONTAINER_ENVIRONMENT_NAME as an output of your "+
					"infrastructure?", serviceConfig.Name, strings.ToUpper(serviceConfig.Name))
		}

		parts := strings.Split(containerEnvName, "/")
		containerEnvName = parts[len(parts)-1]
	}

	return containerEnvName, nil
}

func manifestExists(root string) (bool, error) {
	stat, err := os.Stat(filepath.Join(root, cManifestTemplateFile))
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err == nil && !stat.IsDir() {
		return true, nil
	}

	stat, err = os.Stat(filepath.Join(root, cManifestFile))
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err == nil && !stat.IsDir() {
		return true, nil
	}

	return false, nil
}

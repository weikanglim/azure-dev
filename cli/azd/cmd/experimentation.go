package cmd

import (
	"github.com/azure/azure-dev/cli/azd/cmd/actions"
	"github.com/spf13/cobra"
)

const ExperimentationCommand = "experimentation"
const ExperimentationEvalCommand = "eval"

func experimentationActions(root *actions.ActionDescriptor) *actions.ActionDescriptor {
	group := root.Add(ExperimentationCommand, &actions.ActionDescriptorOptions{
		Command: &cobra.Command{
			Short:  "Manage azd experimentation",
			Long:   "Manage azd experimentation",
			Hidden: true,
		},
	})

	group.Add(ExperimentationEvalCommand, &actions.ActionDescriptorOptions{
		Command: &cobra.Command{
			Short:  "Upload telemetry",
			Long:   "Upload telemetry",
			Hidden: true,
		},
		ActionResolver:   newUploadAction,
		DisableTelemetry: true,
	})

	return group
}

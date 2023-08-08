package telemetry

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/azure/azure-dev/cli/azd/internal/runcontext"
	"github.com/azure/azure-dev/cli/azd/pkg/config"
)

//nolint:lll
const cNotice = `The Azure Developer CLI collects usage data and sends that usage data to Microsoft in order to help us improve your experience.
You can opt-out of telemetry by setting the AZURE_DEV_COLLECT_TELEMETRY environment variable to 'no' in the shell you use.

Read more about Azure Developer CLI telemetry: https://github.com/Azure/azure-dev#data-collection`

// FirstNotice returns the telemetry notice to be shown once to the user, if any.
// The notice is recorded on the filesystem so that it is only shown once.
//
// This currently only returns a notice if the user is running in cloud shell.
func FirstNotice() string {
	// If the user has explicitly opted into or out of telemetry by setting the
	// AZURE_CORE_COLLECT_TELEMETRY environment variable, don't display the
	// notice.
	if _, has := os.LookupEnv(collectTelemetryEnvVar); has {
		return ""
	}

	// We only display the notice in cloud shell.
	if runcontext.IsRunningInCloudShell() && !noticeShown() {
		err := recordNotice()
		if err != nil {
			log.Printf("failed to record first notice: %v", err)
		}
		return cNotice
	}

	return ""
}

func noticeShown() bool {
	notice, err := noticePath()
	if err != nil {
		log.Printf("failed to get notice file path: %v", err)
		// Assume notice not yet shown
		return false
	}

	if _, err := os.Stat(notice); err == nil {
		// Notice previously shown
		return true
	} else if errors.Is(err, fs.ErrNotExist) {
		// Notice unrecorded
		return false
	} else {
		log.Printf("failed to stat notice file: %v", err)
		// If the notice file can't be read, assume not yet shown
		return false
	}
}

func noticePath() (string, error) {
	configDir, err := config.GetUserConfigDir()
	if err != nil {
		log.Printf("failed to get user config dir: %v", err)
		return "", err
	}

	return filepath.Join(configDir, "telemetry-notice"), nil
}

func recordNotice() error {
	noticePath, err := noticePath()
	if err != nil {
		return err
	}
	_, err = os.Create(noticePath)
	return err
}

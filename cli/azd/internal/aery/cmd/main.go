package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	flag "github.com/spf13/pflag"

	"github.com/azure/azure-dev/cli/azd/internal/aery"
)

var (
	subscriptionId = flag.StringP("subscription-id", "s", "", "Azure subscription ID")
	resourceGroup  = flag.StringP("resource-group", "g", "", "Group (Resource group)")
	path           = flag.StringP("file", "f", "", "Path to the resource configuration file")
)

func run() error {
	ctx := context.Background()
	if subscriptionId == nil || *subscriptionId == "" {
		return errors.New("missing --subscription-id/-s")
	}

	if resourceGroup == nil || *resourceGroup == "" {
		return errors.New("missing --resource-group/-g")
	}

	if path == nil || *path == "" {
		return errors.New("missing --file/-f")
	}

	cred, err := azidentity.NewAzureDeveloperCLICredential(nil)
	if err != nil {
		return err
	}

	return aery.Apply(ctx, *path, *subscriptionId, *resourceGroup, cred, aery.ApplyOptions{})
}

func main() {
	flag.Parse()
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

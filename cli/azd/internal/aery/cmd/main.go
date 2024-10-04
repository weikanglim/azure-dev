package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	flag "github.com/spf13/pflag"

	"github.com/azure/azure-dev/cli/azd/internal/aery"
)

// go run main.go -f "/Users/weilim/repos/ai-demo/infra/ai.yaml" -s "faa080af-c1d8-40ad-9cce-e1a450ca5b57" -g rg-weilim-ai-01

var (
	subscriptionId = flag.StringP("subscription-id", "s", "", "Azure subscription ID")
	resourceGroup  = flag.StringP("resource-group", "g", "", "Group (Resource group)")
	path           = flag.StringP("file", "f", "", "Path to the resource configuration file")
	debug          = flag.Bool("debug", false, "Enable debug logging")
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
	log.SetOutput(io.Discard)
	if debug != nil && *debug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.SetOutput(os.Stderr)
	}

	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

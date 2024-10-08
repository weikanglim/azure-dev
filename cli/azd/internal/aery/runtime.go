package aery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	armruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/runtime"
	azruntime "github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/braydonk/yaml"
	yamlToJson "sigs.k8s.io/yaml"
)

type ResourceSpec struct {
	// The name of the resource.
	Name string `yaml:"name"`
	// Optional. The ID of the resource. When specified, this overrides the default ID-to-name translation.
	ID string `yaml:"id"`
	// Optional. The parent resource of the resource.
	// Can be:
	// - name - The name in the same file.
	// - kind/name - A reference to a resource in a different file.
	// (unhandled) - sub/rg/kind/name - A reference to a resource in a different file, possibly existing.
	Parent string `yaml:"parent"`
	// The kind of the resource.
	Kind string `yaml:"kind"`
	// The API version of the resource.
	APIVersion string `yaml:"apiVersion"`
	// The resource properties.
	Spec yaml.Node `yaml:"spec"`
}
type OpType string

const (
	OpTypePut OpType = "HTTP.PUT"
	OpTypeGet OpType = "HTTP.GET"
)

type ExecOp struct {
	Type OpType
	Res  *ResourceSpec
}

// name: test
// id: ""
// kind: Microsoft.CognitiveServices/accounts
// apiVersion: "2023-05-01"

type ApplyOptions struct {
	// ClientOptions contains configuration settings for a client's pipeline.
	ClientOptions *arm.ClientOptions
}

// Apply applies the resource configuration at the given path.
func Apply(
	ctx context.Context,
	path string,
	subscriptionId string,
	resourceGroup string,
	credentials azcore.TokenCredential,
	opt ApplyOptions) error {
	if path == "" {
		return errors.New("path is required")
	}

	if subscriptionId == "" {
		return errors.New("subscriptionId is required")
	}

	stat, err := os.Stat(path)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		//TODO: implement directory support
		return errors.New("currently unsupported")
	}

	if resourceGroup == "" {
		return errors.New("resourceGroup is required when path is a file")
	}

	resources, err := readResources(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	for i := range resources {
		resource := &resources[i]
		// EXP: dynamic parent resolution. Evaluate if this is a good idea.
		if isChildResource(resource.Kind) && resource.Parent == "" {
			log.Println("dynamic-resolve: resolving parent for", resource.Name)
			for j, parent := range resources {
				if i == j {
					continue
				}

				before, after, found := strings.Cut(resource.Kind, parent.Kind)
				log.Printf("dynamic-resolve: cut(%s, %s): %s, %s, %t", resource.Kind, parent.Kind, before, after, found)
				if found && before == "" && len(after) > 1 && after[0] == '/' && !strings.Contains(after[1:], "/") {
					resource.Parent = parent.Kind + "/" + parent.Name
					log.Printf("dynamic-resolve: found parent: %s", resource.Parent)
					break
				}
			}

			if resource.Parent == "" {
				return fmt.Errorf("failed to resolve parent for %s", resource.Name)
			}
		}
	}

	// execute sequentially.
	// TODO: implement parallel execution.
	// TODO: implement dependency resolution.
	pipeline, err := armruntime.NewPipeline("aery", "0.0.1", credentials, azruntime.PipelineOptions{}, opt.ClientOptions)
	if err != nil {
		return fmt.Errorf("failed creating HTTP pipeline: %w", err)
	}

	endpoint := fmt.Sprintf("https://management.azure.com/subscriptions/%s", subscriptionId)
	if resourceGroup != "" {
		endpoint = fmt.Sprintf("%s/resourceGroups/%s", endpoint, resourceGroup)
	}

	execStart := time.Now()
	for _, resource := range resources {
		fmt.Printf("  applying %s...\n", resource.Name)
		resStart := time.Now()
		location := fmt.Sprintf("%s/providers/%s/%s?api-version=%s",
			endpoint, resource.Kind, resource.Name, resource.APIVersion)
		if resource.Parent != "" {
			//IMPROVE: handle full resource IDs
			lastSlash := strings.LastIndex(resource.Kind, "/")
			if len(resource.Parent) < lastSlash || resource.Parent[:lastSlash] != resource.Kind[:lastSlash] {
				return fmt.Errorf("parent resource %s is not a valid parent for resource %s", resource.Parent, resource.Name)
			}
			base := resource.Kind[:lastSlash]
			parentSegment := resource.Parent[lastSlash:]
			childSegment := resource.Kind[lastSlash:]
			location = fmt.Sprintf("%s/providers/%s%s%s/%s?api-version=%s",
				endpoint, base, parentSegment, childSegment, resource.Name, resource.APIVersion)
		}

		req, err := azruntime.NewRequest(ctx, http.MethodPut, location)
		if err != nil {
			return fmt.Errorf("failed creating HTTP request: %w", err)
		}

		yamlBody, err := yaml.Marshal(resource.Spec)
		if err != nil {
			return fmt.Errorf("failed marshalling resource spec: %w", err)
		}
		jsonBody, err := yamlToJson.YAMLToJSON(yamlBody)
		if err != nil {
			return fmt.Errorf("failed converting YAML to JSON: %w", err)
		}

		err = req.SetBody(streaming.NopCloser(bytes.NewReader(jsonBody)), "application/json")
		if err != nil {
			return fmt.Errorf("failed setting body: %w", err)
		}

		resp, err := pipeline.Do(req)
		if err != nil {
			return fmt.Errorf("executing HTTP request: %w", err)
		}

		if !azruntime.HasStatusCode(resp, http.StatusCreated, http.StatusOK) {
			return azruntime.NewResponseError(resp)
		}

		if resp.StatusCode == http.StatusCreated {
			poller, err := azruntime.NewPoller[json.RawMessage](resp, pipeline, nil)
			if err != nil {
				return fmt.Errorf("failed creating poller: %w", err)
			}

			if _, err = poller.PollUntilDone(ctx, &azruntime.PollUntilDoneOptions{Frequency: 1 * time.Second}); err != nil {
				return err
			}
		}

		body, err := azruntime.Payload(resp)
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}

		fmt.Printf("  applied %s in %s\n", resource.Name, time.Since(resStart).Round(100*time.Millisecond))
		log.Println("--------------------------------------------------------------------------------")
		log.Printf("Result of applying resource: %s", location)
		log.Println("--------------------------------------------------------------------------------")
		log.Println(string(body))
		log.Println("--------------------------------------------------------------------------------")
	}
	fmt.Printf("applied all in %s\n", time.Since(execStart).Round(100*time.Millisecond))

	return nil
}

func isChildResource(kind string) bool {
	first := strings.Index(kind, "/")

	if first == -1 || first+1 == len(kind) {
		return false
	}

	return strings.Contains(kind[first+1:], "/")
}

func readResources(filename string) ([]ResourceSpec, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	var docs []ResourceSpec

	for {
		var doc ResourceSpec
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

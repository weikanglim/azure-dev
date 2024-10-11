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
	"path/filepath"
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
	// The explicit name of the resource.
	Name string `yaml:"name"`
	// The alias used to generate an explicit name. If not provided, the name is used as-is.
	Alias string `yaml:"alias"`
	// Optional. The parent resource of the resource.
	// Can be:
	// - name - The name in the same file.
	// - kind/name - A reference to a resource in a different file.
	// (unhandled) - sub/rg/kind/name - A reference to a resource in a different file, possibly existing.
	Parent string `yaml:"parent"`
	// The type of the resource.
	Type string `yaml:"type"`
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

	pipeline, err := armruntime.NewPipeline("aery", "0.0.1", credentials, azruntime.PipelineOptions{}, opt.ClientOptions)
	if err != nil {
		return fmt.Errorf("failed creating HTTP pipeline: %w", err)
	}

	if stat.IsDir() {
		subDef, err := readResourcesFile(filepath.Join(path, "subscription.yaml"))
		if !errors.Is(err, os.ErrNotExist) && err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		subExists := err == nil

		groupDef, err := readResourcesFile(filepath.Join(path, "group.yaml"))
		if !errors.Is(err, os.ErrNotExist) && err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		groupExists := err == nil

		if subExists && groupExists {
			return errors.New("expected to find either subscription.yaml or group.yaml, not both")
		}

		if groupExists {
			if len(groupDef) == 0 {
				return fmt.Errorf("expected to find group definition in %s", filepath.Join(path, "group.yaml"))
			} else if len(groupDef) > 1 {
				return fmt.Errorf("expected a single group definition in %s", filepath.Join(path, "group.yaml"))
			}

			if resourceGroup == "" {
				resourceGroup = groupDef[0].Name
			} else if resourceGroup != groupDef[0].Name {
				return fmt.Errorf("group %s does not match group.yaml: %s", resourceGroup, groupDef[0].Name)
			}

			err = applyResource(ctx, subscriptionId, "", &groupDef[0], pipeline)
			if err != nil {
				return fmt.Errorf("failed applying group %s: %w", groupDef[0].Name, err)
			}
		}

		if subExists {
			if len(subDef) == 0 {
				return fmt.Errorf("expected to find subscription definition in %s", filepath.Join(path, "subscription.yaml"))
			} else if len(subDef) > 1 {
				return fmt.Errorf(
					"expected a single subscription definition in %s", filepath.Join(path, "subscription.yaml"))
			}

			if subscriptionId == "" {
				subscriptionId = subDef[0].Name
			} else if subscriptionId != subDef[0].Name {
				return fmt.Errorf("subscription %s does not match subscription.yaml: %s", subscriptionId, subDef[0].Name)
			}

			// TODO: subscription-id may be different
		}
	}

	paths := []string{}
	if stat.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("reading dir: %w", err)
		}

		for _, ent := range entries {
			if !ent.IsDir() && ent.Name() != "group.yaml" && ent.Name() != "subscription.yaml" {
				paths = append(paths, filepath.Join(path, ent.Name()))
			}
		}
	} else {
		paths = append(paths, path)

		if resourceGroup == "" {
			return errors.New("resourceGroup is required when path is a file")
		}
	}

	for _, p := range paths {
		resources, err := readResourcesFile(p)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		for i := range resources {
			resource := &resources[i]
			// EXP: dynamic parent resolution. Evaluate if this is a good idea.
			if isChildResource(resource.Type) && resource.Parent == "" {
				log.Println("dynamic-resolve: resolving parent for", resource.Name)
				for j, parent := range resources {
					if i == j {
						continue
					}

					before, after, found := strings.Cut(resource.Type, parent.Type)
					log.Printf("dynamic-resolve: cut(%s, %s): %s, %s, %t", resource.Type, parent.Type, before, after, found)
					if found && before == "" && len(after) > 1 && after[0] == '/' && !strings.Contains(after[1:], "/") {
						resource.Parent = parent.Type + "/" + parent.Name
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
		execStart := time.Now()
		for _, resource := range resources {
			err := applyResource(ctx, subscriptionId, resourceGroup, &resource, pipeline)
			if err != nil {
				return fmt.Errorf("failed applying resource %s: %w", resource.Name, err)
			}
		}
		fmt.Printf("applied all in %s\n", time.Since(execStart).Round(100*time.Millisecond))
	}

	return nil
}

func applyResource(
	ctx context.Context,
	subscriptionId string,
	group string,
	resource *ResourceSpec,
	pipeline azruntime.Pipeline) error {
	endpoint := fmt.Sprintf("https://management.azure.com/subscriptions/%s", subscriptionId)
	if group != "" {
		endpoint = fmt.Sprintf("%s/resourceGroups/%s", endpoint, group)
	}

	// TODO: eval loop
	if resource.Name == "" && resource.Alias == "" {
		// TODO: should include file name
		return fmt.Errorf("resource %s must specify either name or alias", resource.Type)
	} else if resource.Name != "" && resource.Alias != "" {
		return fmt.Errorf("resource %s cannot specify both name and alias", resource.Name)
	}

	if resource.Name == "" && resource.Alias != "" {
		uniqueToken, err := UniqueString(subscriptionId, group, resource.Alias)
		if err != nil {
			return fmt.Errorf("generating unique token: %w", err)
		}

		name, err := Name(uniqueToken, *resource)
		if err != nil {
			return fmt.Errorf("generating unique name for alias: %w", err)
		}

		resource.Name = name
	}

	fmt.Printf("  applying %s...\n", resource.Name)
	resStart := time.Now()
	providerSegment := fmt.Sprintf("providers/%s", resource.Type)
	if resource.Type == "Microsoft.Resources/resourceGroups" {
		providerSegment = "resourcegroups"
	}

	location := fmt.Sprintf("%s/%s/%s?api-version=%s", endpoint, providerSegment, resource.Name, resource.APIVersion)

	if resource.Parent != "" {
		//IMPROVE: handle full resource IDs
		lastSlash := strings.LastIndex(resource.Type, "/")
		if len(resource.Parent) < lastSlash || resource.Parent[:lastSlash] != resource.Type[:lastSlash] {
			return fmt.Errorf("parent resource %s is not a valid parent for resource %s", resource.Parent, resource.Name)
		}
		base := resource.Type[:lastSlash]
		parentSegment := resource.Parent[lastSlash:]
		childSegment := resource.Type[lastSlash:]
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

	return nil
}

func isChildResource(kind string) bool {
	first := strings.Index(kind, "/")

	if first == -1 || first+1 == len(kind) {
		return false
	}

	return strings.Contains(kind[first+1:], "/")
}

func readResourcesFile(filename string) ([]ResourceSpec, error) {
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

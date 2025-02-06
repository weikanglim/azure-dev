// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package appgen

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/azure/azure-dev/cli/azd/internal/appdetect"
	"github.com/azure/azure-dev/cli/azd/pkg/osutil"
	"github.com/azure/azure-dev/cli/azd/pkg/yamlnode"
	"github.com/braydonk/yaml"
)

const azurePropertiesFile = "application-azure.yaml"

func resourcesDir(project appdetect.MavenProject) string {
	return filepath.Join(project.Path, "src", "main", "resources")
}

func PropertiesFile(project appdetect.MavenProject) string {
	return filepath.Join(resourcesDir(project), azurePropertiesFile)
}

// GenerateSpringAppProperties generates properties for the maven project.
func GenerateSpringAppProperties(
	ctx context.Context,
	project appdetect.MavenProject) error {
	resourcesDir := filepath.Join(project.Path, "src", "main", "resources")
	err := os.MkdirAll(resourcesDir, osutil.PermissionDirectoryOwnerOnly)
	if err != nil {
		return fmt.Errorf("creating resources directory: %w", err)
	}

	file, err := os.OpenFile(filepath.Join(resourcesDir, azurePropertiesFile), os.O_RDWR|os.O_CREATE, osutil.PermissionFile)
	if err != nil {
		return fmt.Errorf("reading properties file: %w", err)
	}
	defer file.Close()

	var doc yaml.Node
	decoder := yaml.NewDecoder(file)
	decoder.SetScanBlockScalarAsLiteral(true)
	err = decoder.Decode(&doc)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to decode: %w", err)
	}

	if err == io.EOF {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{
			{
				Kind:    yaml.MappingNode,
				Content: []*yaml.Node{},
			},
		}}
	}

	runOnceMap := map[string]struct{}{}
	for _, dep := range project.Dependencies {
		name := dep.GroupId + ":" + dep.ArtifactId
		switch name {
		case "org.postgresql:postgresql",
			"com.azure.spring:spring-cloud-azure-starter-jdbc-postgresql":
			if runOnce(runOnceMap, "postgresql") {
				continue
			}

			properties := map[string]string{
				"spring?.datasource?.url":      "jdbc:postgresql://${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DATABASE}",
				"spring?.datasource?.username": "${POSTGRES_USERNAME}",
				"spring?.datasource?.password": "${POSTGRES_PASSWORD}",
			}
			for key, value := range properties {
				log.Println("Setting", key, value)
				err = yamlnode.Set(&doc, key, &yaml.Node{Kind: yaml.ScalarNode, Value: value})
				if err != nil {
					return fmt.Errorf("setting '%s': %w", key, err)
				}
			}
		}
	}

	// Write modified YAML back to file
	err = file.Truncate(0)
	if err != nil {
		return fmt.Errorf("truncating file: %w", err)
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seeking to start of file: %w", err)
	}

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	// preserve multi-line blocks style
	encoder.SetAssumeBlockAsLiteral(true)
	err = encoder.Encode(&doc)
	if err != nil {
		return fmt.Errorf("failed to encode: %w", err)
	}

	err = file.Close()
	if err != nil {
		return fmt.Errorf("closing file: %w", err)
	}

	return nil
}

// runOnce checks if the name already exists in the map.
// If it does not exist, it adds the binding type to the map.
func runOnce(m map[string]struct{}, name string) bool {
	_, ok := m[name]
	if !ok {
		m[name] = struct{}{}
	}

	return ok
}

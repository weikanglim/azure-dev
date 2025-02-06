// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package appdetect

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/azure/azure-dev/cli/azd/pkg/tools/maven"
)

type javaDetector struct {
	mvnCli       *maven.Cli
	rootProjects []MavenProject
}

func (jd *javaDetector) Language() Language {
	return Java
}

func (jd *javaDetector) DetectProject(ctx context.Context, path string, entries []fs.DirEntry) (*Project, error) {
	for _, entry := range entries {
		if strings.ToLower(entry.Name()) == "pom.xml" {
			pomFile := filepath.Join(path, entry.Name())
			project, err := readMavenProject(ctx, jd.mvnCli, pomFile)
			if err != nil {
				return nil, fmt.Errorf("error reading pom.xml: %w", err)
			}

			if len(project.Modules) > 0 {
				// This is a multi-module project, we will capture the analysis, but return nil
				// to continue recursing
				jd.rootProjects = append(jd.rootProjects, *project)
				return nil, nil
			}

			var currentRoot *MavenProject
			for _, rootProject := range jd.rootProjects {
				// we can say that the project is in the root project if the path is under the project
				if inRoot := strings.HasPrefix(pomFile, rootProject.Path); inRoot {
					currentRoot = &rootProject
				}
			}

			_ = currentRoot // use currentRoot here in the analysis
			result := &Project{
				Language:      Java,
				Path:          path,
				DetectionRule: "Inferred by presence of: pom.xml",
			}

			result, err = analyze(project, result)
			result.RawProject = *project
			if err != nil {
				return nil, fmt.Errorf("detecting dependencies: %w", err)
			}

			return result, nil
		}
	}

	return nil, nil
}

// MavenProject represents the top-level structure of a Maven POM file.
type MavenProject struct {
	XmlName              xml.Name                  `xml:"project"`
	Parent               MavenProjectParent        `xml:"parent"`
	Modules              []string                  `xml:"modules>module"` // Capture the modules
	Dependencies         []MavenDependency         `xml:"dependencies>dependency"`
	DependencyManagement MavenDependencyManagement `xml:"dependencyManagement"`
	Build                MavenBuild                `xml:"build"`

	IsSpringBoot bool
	Path         string
}

// Parent represents the MavenProjectParent POM if this project is a module.
type MavenProjectParent struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
}

// Dependency represents a single Maven MavenDependency.
type MavenDependency struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope,omitempty"`
}

// DependencyManagement includes a list of dependencies that are managed.
type MavenDependencyManagement struct {
	Dependencies []MavenDependency `xml:"dependencies>dependency"`
}

// Build represents the MavenBuild configuration which can contain plugins.
type MavenBuild struct {
	Plugins []MavenPlugin `xml:"plugins>plugin"`
}

// Plugin represents a build MavenPlugin.
type MavenPlugin struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
}

func readMavenProject(ctx context.Context, mvnCli *maven.Cli, filePath string) (*MavenProject, error) {
	effectivePom, err := mvnCli.EffectivePom(ctx, filePath)
	if err != nil {
		return nil, err
	}
	var project MavenProject
	if err := xml.Unmarshal([]byte(effectivePom), &project); err != nil {
		return nil, fmt.Errorf("parsing xml: %w", err)
	}
	project.Path = filepath.Dir(filePath)
	return &project, nil
}

func analyze(mavenProject *MavenProject, project *Project) (*Project, error) {
	// Check if this is a Spring Boot project
	for _, dep := range mavenProject.Build.Plugins {
		if dep.GroupId == "org.springframework.boot" &&
			dep.ArtifactId == "spring-boot-maven-plugin" {
			mavenProject.IsSpringBoot = true
			break
		}
	}

	databaseDepMap := map[DatabaseDep]struct{}{}
	for _, dep := range mavenProject.Dependencies {
		name := dep.GroupId + ":" + dep.ArtifactId
		switch name {
		case "com.mysql:mysql-connector-j":
			databaseDepMap[DbMySql] = struct{}{}
		case "org.postgresql:postgresql",
			"com.azure.spring:spring-cloud-azure-starter-jdbc-postgresql":
			databaseDepMap[DbPostgres] = struct{}{}
		}
	}

	if len(databaseDepMap) > 0 {
		project.DatabaseDeps = slices.SortedFunc(maps.Keys(databaseDepMap),
			func(a, b DatabaseDep) int {
				return strings.Compare(string(a), string(b))
			})
	}

	return project, nil
}

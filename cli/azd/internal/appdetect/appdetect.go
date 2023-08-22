// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package appdetect

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

type Language string

const (
	DotNet     Language = "dotnet"
	Java       Language = "java"
	JavaScript Language = "js"
	TypeScript Language = "ts"
	Python     Language = "python"
)

func (pt Language) Display() string {
	switch pt {
	case DotNet:
		return ".NET"
	case Java:
		return "Java"
	case JavaScript:
		return "JavaScript"
	case TypeScript:
		return "TypeScript"
	case Python:
		return "Python"
	}

	return ""
}

type Dependency string

const (
	JsReact   Dependency = "react"
	JsAngular Dependency = "angular"
	JsVue     Dependency = "vuejs"
	JsJQuery  Dependency = "jquery"

	PyFlask   Dependency = "flask"
	PyDjango  Dependency = "django"
	PyFastApi Dependency = "fastapi"
)

func (f Dependency) Language() Language {
	switch f {
	case JsReact, JsAngular, JsVue, JsJQuery:
		return JavaScript
	}

	return ""
}

func (f Dependency) Display() string {
	switch f {
	case JsReact:
		return "React"
	case JsAngular:
		return "Angular"
	case JsVue:
		return "Vue.js"
	case JsJQuery:
		return "JQuery"
	}

	return ""
}

func (f Dependency) IsWebUIFramework() bool {
	switch f {
	case JsReact, JsAngular, JsVue, JsJQuery:
		return true
	}

	return false
}

// A type of database that is inferred through heuristics while scanning project information.
type DatabaseDep string

const (
	// Database dependencies
	DbPostgres  DatabaseDep = "postgres"
	DbMongo     DatabaseDep = "mongo"
	DbMySql     DatabaseDep = "mysql"
	DbSqlServer DatabaseDep = "sqlserver"
)

func (db DatabaseDep) Display() string {
	switch db {
	case DbPostgres:
		return "PostgreSQL"
	case DbMongo:
		return "MongoDB"
	case DbMySql:
		return "MySQL"
	case DbSqlServer:
		return "SQL Server"
	}

	return ""
}

type Project struct {
	// The language associated with the project.
	Language Language

	// Dependencies scanned in the project.
	Dependencies []Dependency

	// Experimental: Database dependencies inferred through heuristics while scanning dependencies in the project.
	DatabaseDeps []DatabaseDep

	// The path to the project directory.
	Path string

	// A short description of the detection rule applied.
	DetectionRule string

	// If true, the project uses Docker for packaging. This is inferred through the presence of a Dockerfile.
	Docker *Docker
}

func (p *Project) HasWebUIFramework() bool {
	for _, f := range p.Dependencies {
		if f.IsWebUIFramework() {
			return true
		}
	}

	return false
}

type Docker struct {
	Path string
}

type ProjectDetector interface {
	Language() Language
	DetectProject(path string, entries []fs.DirEntry) (*Project, error)
}

var allDetectors = []ProjectDetector{
	// Order here determines precedence when two projects are in the same directory.
	// This is unlikely to occur in practice, but reordering could help to break the tie in these cases.
	&JavaDetector{},
	&DotNetDetector{},
	&PythonDetector{},
	&JavaScriptDetector{},
}

// Detects projects located under an application repository.
func Detect(repoRoot string, options ...DetectOption) ([]Project, error) {
	config := newConfig(options...)
	allProjects := []Project{}

	// Prioritize src directory if it exists
	sourceDir := filepath.Join(repoRoot, "src")
	if ent, err := os.Stat(sourceDir); err == nil && ent.IsDir() {
		projects, err := detectUnder(sourceDir, config)
		if err != nil {
			return nil, err
		}

		if len(projects) > 0 {
			allProjects = append(allProjects, projects...)
		}
	}

	if len(allProjects) == 0 {
		config.ExcludePatterns = append(config.ExcludePatterns, "*/src/")
		projects, err := detectUnder(repoRoot, config)
		if err != nil {
			return nil, err
		}

		if len(projects) > 0 {
			allProjects = append(allProjects, projects...)
		}
	}

	return allProjects, nil
}

// DetectUnder detects projects located under a directory.
func DetectUnder(root string, options ...DetectOption) ([]Project, error) {
	config := newConfig(options...)
	return detectUnder(root, config)
}

// DetectDirectory detects the project located in a directory.
func DetectDirectory(directory string, options ...DetectDirectoryOption) (*Project, error) {
	config := newDirectoryConfig(options...)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	return detectAny(config.detectors, directory, entries)
}

func detectUnder(root string, config detectConfig) ([]Project, error) {
	projects := []Project{}

	walkFunc := func(path string, entries []fs.DirEntry) error {
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		for _, p := range config.ExcludePatterns {
			match, err := doublestar.Match(p, relativePath)
			if err != nil {
				return err
			}
			if match {
				return filepath.SkipDir
			}
		}

		project, err := detectAny(config.detectors, path, entries)
		if err != nil {
			return err
		}

		if project != nil {
			// Once a project is detected, we skip possible inner projects.
			projects = append(projects, *project)
			return filepath.SkipDir
		}

		return nil
	}

	err := walkDirectories(root, walkFunc)
	if err != nil {
		return nil, fmt.Errorf("scanning directories: %w", err)
	}

	return projects, nil
}

// Detects if a directory belongs to any projects.
func detectAny(detectors []ProjectDetector, path string, entries []fs.DirEntry) (*Project, error) {
	log.Printf("Detecting projects in directory: %s", path)
	for _, detector := range detectors {
		project, err := detector.DetectProject(path, entries)
		if err != nil {
			return nil, fmt.Errorf("detecting %s project: %w", string(detector.Language()), err)
		}

		if project != nil {
			log.Printf("Found project %s at %s", project.Language, path)

			// docker is an optional property of a project, and thus is different than other detectors
			docker, err := DetectDockerProject(path, entries)
			if err != nil {
				return nil, fmt.Errorf("detecting docker project: %w", err)
			}
			project.Docker = docker

			return project, nil
		}
	}

	return nil, nil
}

// walkDirFunc is the type of function that is called whenever a directory is visited by WalkDirectories.
//
// path is the directory being visited. entries are the file entries (including directories) in that directory.
type walkDirFunc func(path string, entries []fs.DirEntry) error

// walkDirectories walks the file tree rooted at root, calling fn for each directory in the tree, including root.
// The directories are walked in lexical order.
func walkDirectories(root string, fn walkDirFunc) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}

	return walkDirRecursive(root, fs.FileInfoToDirEntry(info), fn)
}

func walkDirRecursive(path string, d fs.DirEntry, fn walkDirFunc) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	err = fn(path, entries)
	if errors.Is(err, filepath.SkipDir) {
		// skip the directory
		return nil
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dir := filepath.Join(path, entry.Name())
			err = walkDirRecursive(dir, entry, fn)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

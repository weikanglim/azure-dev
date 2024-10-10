package project

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/pkg/encoding/yaml"
	"github.com/azure/azure-dev/cli/azd/internal/aerygen"
	"github.com/azure/azure-dev/cli/azd/pkg/osutil"
	"github.com/azure/azure-dev/cli/azd/resources"
)

type Fs interface {
	MkdirAll(dir string, perm os.FileMode) error
	WriteFile(name string, data []byte, perm os.FileMode) error
}

type GenerateOptions struct {
	// Root is the root directory to write the generated files to.
	Root string
	// fs is the file system to write the generated files to.
	fs Fs
}

// genContext is the context for generating a resource definition.
type genContext struct {
	// The actual name of the resource after name translation
	Name string `json:"name"`
	// The tags to apply
	Tags map[string]string `json:"tags"`
}

// GenerateResourceDefinitions generates resource definition files from a service definition.
func GenerateResourceDefinitions(
	resConfig *ResourceConfig,
	dir string, // could be resource group dir
	options GenerateOptions) error {
	ctx := cuecontext.New()
	var value cue.Value

	switch resConfig.Type {
	case ResourceTypeOpenAiModel:
		cueFile, err := resources.AeryGen.ReadFile("aery-gen/ai.cue")
		if err != nil {
			return fmt.Errorf("error reading ai.cue: %v", err)
		}

		value = ctx.CompileBytes(cueFile)
	default:
		return fmt.Errorf("unsupported resource type: %v", resConfig.Type)
	}

	// add encoding for resConfig
	val := value.FillPath(cue.ParsePath("input"), *resConfig)
	if err := val.Err(); err != nil {
		return fmt.Errorf("error filling input: %v", err)
	}

	iter, err := val.LookupPath(cue.ParsePath("output")).List()
	if err != nil {
		return fmt.Errorf("error getting output list: %v", err)
	}

	i := 0
	for iter.Next() {
		outputVal := iter.Value()

		name, err := aerygen.Name(outputVal)
		if err != nil {
			return fmt.Errorf("error translating name: %w", err)
		}

		genCtx := genContext{}
		genCtx.Name = name
		genCtx.Tags = resConfig.Tags
		// improve: we can add more context here

		// set the name
		val = val.FillPath(cue.ParsePath(fmt.Sprintf("ctx[%d]", i)), genCtx)
		i++
	}

	contents, err := val.MarshalJSON()
	if err != nil {
		return fmt.Errorf("error marshaling result: %v", err)
	}
	log.Println("cue evaluation:")
	log.Println(string(contents))

	// What do I want to do:
	// 1. Generate a single file with multiple YAML documents
	// 2. Want the generation also be a single file
	// 3. Want the name generation to be done in go

	// Solution:

	// Loop through each thing, fill the name.

	// Marshal the result back to CUE syntax
	bytes, err := yaml.MarshalStream(val.LookupPath(cue.ParsePath("output")))
	if err != nil {
		return fmt.Errorf("error marshaling result: %v", err)
	}

	fs := options.fs
	if fs == nil {
		fs = &writableFS{}
	}

	if options.Root != "" {
		dir = filepath.Join(options.Root, dir)
	}

	if dir != "" {
		if err := fs.MkdirAll(dir, osutil.PermissionDirectory); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
	}

	path := "ai.yaml"
	if dir != "" {
		path = filepath.Join(dir, path)
	}
	log.Printf("aery-gen: writing: %s", path)
	err = fs.WriteFile(path, []byte(bytes), osutil.PermissionFileOwnerOnly)
	if err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	return nil
}

type writableFS struct{}

func (*writableFS) MkdirAll(dir string, perm os.FileMode) error {
	return os.MkdirAll(dir, perm)
}

func (*writableFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

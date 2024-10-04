package project

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/pkg/encoding/yaml"
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
		if value.Err() != nil {
			return fmt.Errorf("error building CUE value: %v", value.Err())
		}
	default:
		return fmt.Errorf("unsupported resource type: %v", resConfig.Type)
	}

	// add encoding for resConfig
	val := value.FillPath(cue.ParsePath("input"), *resConfig)
	if val.Err() != nil {
		return fmt.Errorf("error filling value: %v", val.Err())
	}

	curr, err := val.LookupPath(cue.ParsePath("input")).MarshalJSON()
	if err != nil {
		return fmt.Errorf("error marshaling value: %v", err)
	}

	log.Println(curr)

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

package project

import (
	"fmt"
	"log"
	"os"
	"path"
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
	//TODO: Remove
	Token string
	// Root is the root directory to write the generated files to.
	Root string
	// fs is the file system to write the generated files to.
	fs Fs
}

// genContext is the context for generating a resource definition.
type genContext struct {
	// The alias of the resource
	Alias string `json:"alias"`
	// The tags to apply
	Tags map[string]string `json:"tags"`
}

// "type": "Microsoft.Resources/resourceGroups",
// "apiVersion": "2022-09-01",
// "name": "[format('rg-{0}', parameters('environmentName'))]",
// "location": "[parameters('location')]",
// "tags": "[variables('tags')]"

func GenerateResourceGroup(resConfig *ResourceConfig, dir string, options GenerateOptions) error {
	ctx := cuecontext.New()
	// first determine if we're inside a group or subscription
	if options.fs == nil {
		options.fs = &writableFS{}
	}

	if options.Root != "" {
		dir = filepath.Join(options.Root, dir)
	}

	if dir != "" {
		if err := options.fs.MkdirAll(dir, osutil.PermissionDirectory); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
	}

	err := generateFile(ctx, resConfig, "group.cue", filepath.Join(dir, "group.yaml"), options)
	if err != nil {
		return fmt.Errorf("error generating logs.yaml: %v", err)
	}

	return nil
}

// GenerateResourceDefinitions generates resource definition files from a service definition.
func GenerateResourceDefinitions(
	resConfig *ResourceConfig,
	dir string, // could be resource group dir
	options GenerateOptions) error {
	ctx := cuecontext.New()
	// first determine if we're inside a group or subscription
	if options.fs == nil {
		options.fs = &writableFS{}
	}

	if options.Root != "" {
		dir = filepath.Join(options.Root, dir)
	}

	if dir != "" {
		if err := options.fs.MkdirAll(dir, osutil.PermissionDirectory); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
	}

	switch resConfig.Type {
	case ResourceTypeOpenAiModel:
		err := generateFile(ctx, resConfig, "ai.cue", filepath.Join(dir, "ai.yaml"), options)
		if err != nil {
			return fmt.Errorf("error generating ai.yaml: %v", err)
		}
	case ResourceTypeHostContainerApp:
		err := generateFile(ctx, resConfig, "logs.cue", filepath.Join(dir, "logs.yaml"), options)
		if err != nil {
			return fmt.Errorf("error generating logs.yaml: %v", err)
		}
	default:
		return fmt.Errorf("unsupported resource type: %v", resConfig.Type)
	}

	return nil
}

func generateFile(
	cueCtx *cue.Context,
	input *ResourceConfig,
	srcPath string,
	destPath string,
	options GenerateOptions) error {
	root := "aery-gen"
	cueFile, err := resources.AeryGen.ReadFile(path.Join(root, srcPath))
	if err != nil {
		return fmt.Errorf("error reading logs.cue: %v", err)
	}

	value := cueCtx.CompileBytes(cueFile)

	// add encoding for resConfig
	val := value.FillPath(cue.ParsePath("input"), *input)
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

		alias, err := aerygen.Alias(outputVal)
		if err != nil {
			return fmt.Errorf("getting alias: %w", err)
		}

		genCtx := genContext{}
		genCtx.Alias = alias
		genCtx.Tags = input.Tags
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

	// Marshal the result back to CUE syntax
	bytes, err := yaml.MarshalStream(val.LookupPath(cue.ParsePath("output")))
	if err != nil {
		return fmt.Errorf("error marshaling result: %v", err)
	}

	fs := options.fs
	log.Printf("aery-gen: writing: %s", destPath)
	err = fs.WriteFile(destPath, []byte(bytes), osutil.PermissionFileOwnerOnly)
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

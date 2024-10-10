package aerygen

import (
	"fmt"
	"log"
	"strings"

	"cuelang.org/go/cue"
	"github.com/azure/azure-dev/cli/azd/resources"
	"github.com/braydonk/yaml"
)

// Azure naming conventions.
type AzureNames struct {
	// Conventions groupe by resource types, with their associated kinds.
	Types map[string][]ResourceKind `yaml:"resourceTypes"`
}

// A resource kind. A resource type can have multiple kinds.
// In the default case, a resource type has a single kind that is empty.
type ResourceKind struct {
	// Display name of the resource kind.
	Name string `yaml:"name"`
	// The kind of the resource. Empty for resources with just types but no kinds.
	Kind string `yaml:"kind,omitempty"`
	// A custom kind. The value that determines the kind is embedded somewhere as a property in the resource JSON.
	CustomKind CustomKind `yaml:"customKind,omitempty"`
	// Short name abbreviation for new resources.
	Abbreviation string `yaml:"abbreviation"`
	// The rules for naming a resource.
	NamingRules NamingRules `yaml:"namingRules,omitempty"`
}

// The rules for naming a resource.
type NamingRules struct {
	MinLength       int    `yaml:"minLength,omitempty"`
	MaxLength       int    `yaml:"maxLength,omitempty"`
	UniquenessScope string `yaml:"uniquenessScope"`
	Regex           string `yaml:"regex"`
	WordSeparator   string `yaml:"wordSeparator"`

	RestrictedChars RestrictedChars `yaml:"restrictedChars,omitempty"`
	Messages        Messages        `yaml:"messages,omitempty"`
}

type CustomKind struct {
	PropertyPath string `yaml:"propertyPath,omitempty"`
	Value        string `yaml:"value,omitempty"`
}

type Messages struct {
	OnSuccess string `yaml:"onSuccess,omitempty"`
	OnFailure string `yaml:"onFailure,omitempty"`
}

type RestrictedChars struct {
	Global      string `yaml:"global,omitempty"`
	Prefix      string `yaml:"prefix,omitempty"`
	Suffix      string `yaml:"suffix,omitempty"`
	Consecutive string `yaml:"consecutive,omitempty"`
}

var names AzureNames

func init() {
	err := yaml.Unmarshal(resources.AzureNames, &names)
	if err != nil {
		log.Panic("failed marshaling azure resource names %w", err)
	}
}

var ErrNotFound = fmt.Errorf("naming translation for resource type not found")

// Name returns the abbreviation for a resource definition.
func Name(resourceDefinition cue.Value) (string, error) {
	existingName := resourceDefinition.LookupPath(cue.ParsePath("name"))
	if val, err := existingName.String(); err == nil {
		return val, nil
	}

	resourceType, err := resourceDefinition.LookupPath(cue.ParsePath("type")).String()
	if err != nil {
		return "", fmt.Errorf("error getting resource.type: %w", err)
	}

	resTypeNames, ok := names.Types[resourceType]
	if !ok {
		return "", fmt.Errorf("%s: %w", resourceType, ErrNotFound)
	}

	//feat: Expand this to support uniqueness constraints and generate more friendly names when
	// resources are unique to the group scope

	// single kind, short-circuit here to return abbreviation
	if len(resTypeNames) == 1 && resTypeNames[0].Kind == "" {
		return resTypeNames[0].Abbreviation, nil
	}

	// fallback for the resource type abbreviation
	nameForType := ""

	// evaluate each kind rule to see if applies
	for _, resKindNames := range resTypeNames {
		if resKindNames.Kind == "" {
			nameForType = resKindNames.Abbreviation
			continue
		}

		kindPath := "kind"
		kindVal := resKindNames.Kind
		if resKindNames.CustomKind.PropertyPath != "" {
			kindPath = resKindNames.CustomKind.PropertyPath
			kindVal = resKindNames.CustomKind.Value
		}

		lookupKind := resourceDefinition.LookupPath(cue.ParsePath(
			fmt.Sprintf("spec.%s", kindPath)))
		if lookupKind.Exists() {
			resourceDefKind, err := lookupKind.String()
			if err != nil {
				return "", fmt.Errorf("error getting resource.%s: %w", kindPath, err)
			}

			// case-insensitive, partial match
			if strings.Contains(strings.ToLower(resourceDefKind), strings.ToLower(kindVal)) {
				return resKindNames.Abbreviation, nil
			}
		}
	}

	return nameForType, nil
}

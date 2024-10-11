package azure

import (
	"log"

	"github.com/azure/azure-dev/cli/azd/resources"
	"github.com/braydonk/yaml"
)

// Azure naming conventions.
type AzureNames struct {
	// Conventions grouped by resource types, with their associated kinds.
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

// Names for Azure resources.
var Names AzureNames

func init() {
	err := yaml.Unmarshal(resources.AzureNames, &Names)
	if err != nil {
		log.Panic("failed marshaling azure resource names %w", err)
	}
}

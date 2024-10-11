package aerygen

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"github.com/azure/azure-dev/cli/azd/pkg/azure"
)

var ErrNotFound = fmt.Errorf("naming translation for resource type not found")

// Name returns the a suitable name for the given resource.
//
// The name is currently generated by:
//   - {alias}[-]{token}
func Name(token string, resourceDefinition cue.Value) (string, error) {
	existingName := resourceDefinition.LookupPath(cue.ParsePath("name"))
	if val, err := existingName.String(); err == nil {
		return val, nil
	}

	alias := ""
	existingAlias := resourceDefinition.LookupPath(cue.ParsePath("alias"))
	if val, err := existingAlias.String(); err == nil {
		alias = val
	}

	resourceType, err := resourceDefinition.LookupPath(cue.ParsePath("type")).String()
	if err != nil {
		return "", fmt.Errorf("error getting resource.type: %w", err)
	}

	resTypeNames, ok := azure.Names.Types[resourceType]
	if !ok {
		return "", fmt.Errorf("%s: %w", resourceType, ErrNotFound)
	}

	// fallback for the resource type abbreviation
	kind, err := matchResourceKind(resourceDefinition, resTypeNames)
	if err != nil {
		return "", fmt.Errorf("error getting resource kind: %w", err)
	}

	if kind == nil {
		return "", fmt.Errorf("evaluating kind: %s: %w", resourceType, ErrNotFound)
	}

	separator := "-"
	if strings.Contains(kind.NamingRules.RestrictedChars.Global, "-") {
		separator = ""
	}

	if alias == "" {
		alias = kind.Abbreviation
	}

	return fmt.Sprintf("%s%s%s",
		alias,
		separator,
		token), nil
}

// Alias returns the alias for the given resource.
//
// The alias, if not already set, is determined by the default abbreviation of the resource type and kind.
func Alias(resourceDefinition cue.Value) (string, error) {
	existingName := resourceDefinition.LookupPath(cue.ParsePath("name"))
	if val, err := existingName.String(); err == nil {
		return val, nil
	}

	resourceType, err := resourceDefinition.LookupPath(cue.ParsePath("type")).String()
	if err != nil {
		return "", fmt.Errorf("error getting resource.type: %w", err)
	}

	resTypeNames, ok := azure.Names.Types[resourceType]
	if !ok {
		return "", fmt.Errorf("%s: %w", resourceType, ErrNotFound)
	}

	// fallback for the resource type abbreviation
	kind, err := matchResourceKind(resourceDefinition, resTypeNames)
	if err != nil {
		return "", fmt.Errorf("error getting resource kind: %w", err)
	}

	if kind == nil {
		return "", fmt.Errorf("evaluating kind: %s: %w", resourceType, ErrNotFound)
	}

	return kind.Abbreviation, nil
}

// matchResourceKind returns the kind of the resource definition.
func matchResourceKind(
	resourceDefinition cue.Value,
	kinds []azure.ResourceKind) (*azure.ResourceKind, error) {
	var foundKind *azure.ResourceKind

	for _, resKind := range kinds {
		if resKind.Kind == "" {
			foundKind = &resKind
			continue
		}

		// evaluate the kind rule to see if applies
		kindPath := "kind"
		kindVal := resKind.Kind
		if resKind.CustomKind.PropertyPath != "" {
			kindPath = resKind.CustomKind.PropertyPath
			kindVal = resKind.CustomKind.Value
		}

		lookupKind := resourceDefinition.LookupPath(cue.ParsePath(
			fmt.Sprintf("spec.%s", kindPath)))
		if lookupKind.Exists() {
			resourceDefKind, err := lookupKind.String()
			if err != nil {
				return nil, fmt.Errorf("error getting resource.%s: %w", kindPath, err)
			}

			// case-insensitive, partial match
			if strings.Contains(strings.ToLower(resourceDefKind), strings.ToLower(kindVal)) {
				foundKind = &resKind
				break
			}
		}
	}

	return foundKind, nil
}

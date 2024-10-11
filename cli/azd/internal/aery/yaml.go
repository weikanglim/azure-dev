package aery

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/braydonk/yaml"
)

var ErrNodeNotFound = fmt.Errorf("path not found")

// GetNode retrieves a node from a YAML document using a dot-separated path.
//
// The path can contain array indexing using square brackets, e.g. "root.array[1].key".
func GetNode(root *yaml.Node, path string) (*yaml.Node, error) {
	parts := strings.Split(path, ".")
	// add array indexing as integer parts
	expanded, err := expandArrays(parts)
	if err != nil {
		return nil, err
	}

	found, err := find(root, expanded)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, path)
	}

	return found, nil
}

func find(current *yaml.Node, parts []any) (*yaml.Node, error) {
	if len(parts) == 0 {
		return current, nil
	}

	seek, _ := parts[0].(string)
	idx, isArray := parts[0].(int)

	switch current.Kind {
	case yaml.DocumentNode:
		return find(current.Content[0], parts)
	case yaml.MappingNode:
		for i := 0; i < len(current.Content); i += 2 {
			if current.Content[i].Value == seek {
				return find(current.Content[i+1], parts[1:])
			}
		}
	case yaml.SequenceNode:
		if isArray && idx < len(current.Content) {
			return find(current.Content[idx], parts[1:])
		}
	}

	return nil, ErrNodeNotFound
}

func expandArrays(parts []string) (expanded []any, err error) {
	expanded = make([]interface{}, 0, len(parts))
	for _, s := range parts {
		before, after := cutBrackets(s)
		expanded = append(expanded, before)

		if len(after) > 0 {
			content := after[1 : len(after)-1]
			idx, err := strconv.Atoi(content)
			if err != nil {
				return nil, fmt.Errorf("invalid array index: %s in %s", content, after)
			}

			expanded = append(expanded, idx)
		}
	}

	return expanded, nil
}

// cutBrackets splits a string into two parts, before the brackets, and after the brackets.
func cutBrackets(s string) (before string, after string) {
	if len(s) > 0 && s[len(s)-1] == ']' { // reverse check for faster exit
		for i := len(s) - 1; i >= 0; i-- {
			if s[i] == '[' {
				return s[:i], s[i:]
			}
		}
	}

	return s, ""
}

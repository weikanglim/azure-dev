package aery

import (
	"testing"

	"github.com/braydonk/yaml"
)

func TestGetNode(t *testing.T) {
	yamlStr := `
root:
  nested:
    key: value
  array:
    - item1
    - item2
    - item3
  mixedArray:
    - stringItem
    - nestedObj:
        deepKey: deepValue
`

	var root yaml.Node
	err := yaml.Unmarshal([]byte(yamlStr), &root)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected string
		wantErr  bool
	}{
		{"Simple path", "root.nested.key", "value", false},
		{"Array index", "root.array[1]", "item2", false},
		{"Nested array object", "root.mixedArray[1].nestedObj.deepKey", "deepValue", false},
		{"Non-existent path", "root.nonexistent", "", true},
		{"Invalid array index", "root.array[5]", "", true},
		{"Invalid path format", "root.array.[1]", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := GetNode(&root, tt.path)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if node.Kind != yaml.ScalarNode {
					t.Errorf("GetNode() returned non-scalar node '%d' for path %s", node.Kind, tt.path)
				}
				if node.Value != tt.expected {
					t.Errorf("GetNode() = %v, want %v", node.Value, tt.expected)
				}
			}
		})
	}
}

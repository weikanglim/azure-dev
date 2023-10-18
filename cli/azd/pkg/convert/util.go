package convert

import (
	"encoding/json"
	"fmt"
)

// Converts a pointer to a value type
// If the ptr is nil returns default value, otherwise the value of value of the pointer
func ToValueWithDefault[T any](ptr *T, defaultValue T) T {
	if ptr == nil {
		return defaultValue
	}

	return *ptr
}

// Returns a pointer for the specified value
func RefOf[T any](value T) *T {
	return &value
}

// Attempts to convert the specified value to a string, otherwise returns the default value
func ToStringWithDefault(value any, defaultValue string) string {
	if str, ok := value.(string); ok {
		return str
	}

	return defaultValue
}

// Converts the specified value to a map.
func ToMap(value any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}

	jsonValue, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to convert value to json: %w", err)
	}

	mapValue := map[string]any{}
	if err := json.Unmarshal(jsonValue, &mapValue); err != nil {
		return nil, fmt.Errorf("failed to convert value to map: %w", err)
	}

	return mapValue, nil
}

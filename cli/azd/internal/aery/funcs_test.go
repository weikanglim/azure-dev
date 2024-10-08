package aery

import (
	"testing"
)

func TestUniqueString(t *testing.T) {
	tests := []struct {
		name   string
		inputs []string
		want   string
	}{
		{"empty", []string{""}, "aaaaaaaaaaaaa"},
		{"single char", []string{"a"}, "eveiun73364hy"},
		{"spaces", []string{"     "}, "ywy5tpb565m7y"},
		{"sub-id", []string{"sub-id"}, "m3qsgok2tj3gw"},
		{"sub-id env-name location", []string{"sub-id", "env-name", "location"}, "dshmwnunpaa2a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := UniqueString(tt.inputs...); got != tt.want {
				t.Errorf("UniqueString() = %v, want %v", got, tt.want)
			}
		})
	}
}

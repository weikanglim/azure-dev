package aery

import "testing"

func TestUniqueString(t *testing.T) {
	tests := []struct {
		name   string
		inputs []string
		want   string
	}{
		{
			name:   "empty",
			inputs: []string{""},
			want:   "aaaaaaaaaaaaa",
		},
		{
			name:   "single",
			inputs: []string{"sub-id"},
			want:   "m3qsgok2tj3gw",
		},
		{
			name: "multiple",
			inputs: []string{
				"sub-id",
				"env-name",
				"location",
			},
			want: "dshmwnunpaa2a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := UniqueString(tt.inputs...); got != tt.want {
				t.Errorf("UniqueString() = %v, want %v", got, tt.want)
			}
		})
	}
}

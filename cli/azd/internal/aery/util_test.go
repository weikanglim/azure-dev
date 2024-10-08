package aery

import (
	"strconv"
	"testing"
)

func TestMurmurHash64(t *testing.T) {
	tests := []struct {
		input        string
		seed         uint32
		expectedHash uint64
	}{
		{"", 0x00000000, 0x0000000000000000},
		{"", 0x00000001, 0x3CD62B54B6E5772D},
		{"", 0xffffffff, 0xCE0306DC23CC64F0},
		{"test", 0x00000000, 0x884C5DDEBCA14D93},
		{"test", 0x9747b28c, 0x4FDB466423FF1947},
		{"Hello, world!", 0x00000000, 0x894A6F1657A9D3D1},
		{"Hello, world!", 0x9747b28c, 0xAA57E8E1C9FC2007},
		{"The quick brown fox jumps over the lazy dog", 0x00000000, 0xFD8D57877D794DD9},
		{"The quick brown fox jumps over the lazy dog", 0x9747b28c, 0x3519BF26ED840E4D},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := MurmurHash64([]byte(tt.input), tt.seed); got != tt.expectedHash {
				t.Errorf("MurmurHash64() = %v, want %v", got, tt.expectedHash)
			}
		})
	}
}

func Benchmark64Sizes(b *testing.B) {
	buf := make([]byte, 8192)
	for length := 32; length <= cap(buf); length *= 2 {
		b.Run(strconv.Itoa(length), func(b *testing.B) {
			buf = buf[:length]
			b.SetBytes(int64(length))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				MurmurHash64(buf, 0)
			}
		})
	}
}

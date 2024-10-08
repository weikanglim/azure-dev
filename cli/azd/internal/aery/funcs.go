package aery

import (
	"fmt"
	"strings"
)

// UniqueString generates a unique 13-character, base32-encoded string from the input strings.
//
// The generated string is deterministic and will always be the same for the same input strings.
func UniqueString(input ...string) (string, error) {
	if len(input) == 0 {
		return "", fmt.Errorf("uniqueString requires at least one input")
	}
	inputStr := strings.Join(input, "-")
	hash := MurmurHash64([]byte(inputStr), 0)
	return strings.ToLower(base32EncodeLen13(hash)), nil
}

// base32EncodeLen13 encodes the input64 into a 13-character base32-encoded string.
func base32EncodeLen13(input64 uint64) string {
	charset := "abcdefghijklmnopqrstuvwxyz234567"
	var sb strings.Builder
	// only 13-characters are encoded
	for index := 0; index < 13; index++ {
		sb.WriteByte(charset[(int)(input64>>59)])
		input64 <<= 5
	}

	return sb.String()
}

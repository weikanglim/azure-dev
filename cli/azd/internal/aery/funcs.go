package aery

import (
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/spaolacci/murmur3"
)

// on2welljmqaaaaaaaaaaaaa
// on2welljmqaaa

var murmur64 = murmur3.New64()

func UniqueString(input ...string) (string, error) {
	if len(input) == 0 {
		return "", fmt.Errorf("uniqueString requires at least one input")
	}
	inputStr := strings.Join(input, "-")
	hash := murmur64.Sum([]byte(inputStr))
	return strings.ToLower(base32Encode(hash)), nil
}

// copied from Base32Encode -- there may be a more efficient way to do this
func base32Encode(input []byte) string {
	charset := "abcdefghijklmnopqrstuvwxyz234567"
	var sb strings.Builder

	input64 := binary.BigEndian.Uint64(input)
	// input64, err := convertBytesToInt64(input)
	// if err != nil {
	// 	panic(err)
	// }

	// only 13-characters are encoded
	for index := 0; index < 13; index++ {
		sb.WriteByte(charset[(int)(input64>>59)])
		input64 <<= 5
	}

	return sb.String()
}

func stdBase32Encode(value []byte) string {
	// Encode byte slice to Base32
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	return encoding.EncodeToString(value)
}

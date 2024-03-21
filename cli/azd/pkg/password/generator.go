// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package password

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
)

const (
	LowercaseLetters = "abcdefghijklmnopqrstuvwxyz"
	UppercaseLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	Digits           = "0123456789"
	Symbols          = "~!@#$%^&*()_+`-={}|[]\\:\"<>?,./"
	LettersAndDigits = LowercaseLetters + UppercaseLetters + Digits
)

type PasswordComposition struct {
	NumLowercase, NumUppercase, NumDigits, NumSymbols uint
}

// FromAlphabet generates a password of a given length, using only characters from the given alphabet (which should
// be a string with no duplicates)
func FromAlphabet(alphabet string, length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("Empty passwords are insecure")
	}

	chars := make([]byte, length)
	var pos uint = 0
	if err := addRandomChars(chars, &pos, uint(length), alphabet); err != nil {
		return "", err
	}

	return string(chars), nil
}

// Generate password consisting of given number of lowercase letters, uppercase letters, digits, and "symbol" characters.
func Generate(cmp PasswordComposition) (string, error) {
	length := cmp.NumLowercase + cmp.NumUppercase + cmp.NumDigits + cmp.NumSymbols
	if length == 0 {
		return "", fmt.Errorf("Empty passwords are insecure")
	}

	chars := make([]byte, length)
	var pos uint = 0
	var err error

	err = addRandomChars(chars, &pos, cmp.NumLowercase, LowercaseLetters)
	if err != nil {
		return "", err
	}
	err = addRandomChars(chars, &pos, cmp.NumUppercase, UppercaseLetters)
	if err != nil {
		return "", err
	}
	err = addRandomChars(chars, &pos, cmp.NumDigits, Digits)
	if err != nil {
		return "", err
	}
	err = addRandomChars(chars, &pos, cmp.NumSymbols, Symbols)
	if err != nil {
		return "", err
	}

	err = Shuffle(chars)
	if err != nil {
		return "", err
	}
	return string(chars), nil
}

type RandomStringComposition struct {
	// The desired length of the generated string.
	// When unset, the length of the generated string is MinLower + MinUpper + MinNumeric + MinSpecial.
	Length uint

	// If true, lowercase characters are allowed
	Lower    bool
	MinLower uint

	// If true, uppercase characters are allowed
	Upper    bool
	MinUpper uint

	// If true, numeric characters are allowed
	Numeric    bool
	MinNumeric uint

	// If true, special characters are allowed
	Special    bool
	MinSpecial uint
}

func RandomString(input RandomStringComposition) (string, error) {
	var length uint
	if input.Length > 0 {
		length = input.Length
	} else {
		length = input.MinLower + input.MinUpper + input.MinNumeric + input.MinSpecial
		if length < 1 {
			return "", errors.New("a fixed length or minimum lengths must be specified")
		}
	}

	requirements := []struct {
		allowed bool
		min     uint
		charSet string
	}{
		{input.Lower, input.MinLower, LowercaseLetters},
		{input.Upper, input.MinUpper, UppercaseLetters},
		{input.Numeric, input.MinNumeric, Digits},
		{input.Special, input.MinSpecial, Symbols},
	}

	chars := make([]byte, length)
	allowedCharSet := ""
	var pos uint = 0
	// Fill based on charset length requirements
	for _, r := range requirements {
		if r.min > 0 {
			if err := addRandomChars(chars, &pos, r.min, r.charSet); err != nil {
				return "", err
			}
		}

		if r.allowed {
			allowedCharSet += r.charSet
		}
	}

	if len(allowedCharSet) == 0 {
		return "", errors.New("some characters must be allowed")
	}

	// Fill remaining
	if pos < length-1 {
		if err := addRandomChars(chars, &pos, length-pos, allowedCharSet); err != nil {
			return "", err
		}
	}

	err := Shuffle(chars)
	if err != nil {
		return "", err
	}
	return string(chars), nil
}

func addRandomChars(buf []byte, pos *uint, count uint, choices string) error {
	var i uint
	for i = 0; i < count; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(choices))))
		if err != nil {
			return err
		}
		buf[i+*pos] = choices[n.Int64()]
	}

	*pos += count
	return nil
}

package common

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/rand"
	"unicode/utf16"
)

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

var hexRunes = []rune("0123456789abcdefabcdef")

func RandomHexString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(hexRunes))]
	}
	return string(b)
}

func UTF16ToByteArray(wideString []uint16) []byte {
	byteArray := make([]byte, len(wideString)*2)
	for i, b := range wideString {
		byteArray[(i*2)+0] = byte(b >> 8)
		byteArray[(i*2)+1] = byte(b >> 0)
	}
	return byteArray
}

func GetString(buf []byte) (string, error) {
	before, _, ok := bytes.Cut(buf, []byte{0})

	if !ok {
		return "", errors.New("buf is not null-terminated")
	}

	return string(before), nil
}

func GetWideString(buf []byte, byteOrder binary.ByteOrder) (string, error) {
	var utf16String []uint16
	for i := 0; i < len(buf)/2; i++ {
		if buf[i*2] == 0 && buf[i*2+1] == 0 {
			break
		}

		utf16String = append(utf16String, byteOrder.Uint16(buf[i*2:i*2+2]))
	}
	return string(utf16.Decode(utf16String)), nil
}

// IsUppercaseAlphanumeric checks if the given string is composed exclusively of uppercase alphanumeric characters.
func IsUppercaseAlphanumeric(str string) bool {
	strLength := len(str)

	if strLength == 0 {
		return false
	}

	for i := range strLength {
		c := str[i]

		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') {
			return false
		}
	}

	return true
}

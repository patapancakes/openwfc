package qr2

import (
	"encoding/base64"
)

func encode(data []byte) string {
	padded := make([]byte, ((len(data)+2)/3)*3)
	copy(padded, data)

	return base64.StdEncoding.EncodeToString(padded)
}

func encrypt(key []byte, data []byte) []byte {
	var state [256]byte

	swap := func(a *byte, b *byte) {
		swap := *a
		*a = *b
		*b = swap
	}

	for counter := range state {
		state[counter] = byte(counter)
	}

	var x, y uint8
	for counter := range state {
		y = key[x] + state[counter] + y
		x = (x + 1) % uint8(len(key))
		swap(&state[counter], &state[y])
	}

	x = 0
	y = 0

	var xorIdx uint8
	for counter := range data {
		x = x + data[counter] + 1
		y = state[x] + y
		swap(&state[x], &state[y])
		xorIdx = state[x] + state[y]
		data[counter] ^= state[xorIdx]
	}

	return data
}

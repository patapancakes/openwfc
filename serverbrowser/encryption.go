package serverbrowser

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"slices"
)

type CryptState struct {
	cards      [256]byte
	rotor      byte
	ratchet    byte
	avalanche  byte
	lastPlain  byte
	lastCipher byte
}

func EncryptTypeX(key []byte, challenge []byte, data []byte) []byte {
	var state CryptState
	return append(state.CreateHeader(key, challenge), state.Encrypt(data)...)
}

// also initializes crypto state
func (s *CryptState) CreateHeader(key, challenge []byte) []byte {
	header := new(bytes.Buffer)

	const randLen = 5
	header.WriteByte((randLen + 2) ^ 0xEC)                // random bytes + backend game flags length
	binary.Write(header, binary.BigEndian, uint16(0))     // backend game flags
	io.Copy(header, io.LimitReader(rand.Reader, randLen)) // write randLen bytes

	const keyLen = 8
	header.WriteByte(keyLen ^ 0xEA)                      // key length
	io.Copy(header, io.LimitReader(rand.Reader, keyLen)) // write keyLen bytes

	s.initKey(key, challenge, header.Bytes()[1+2+randLen+1:]) // rand length byte + backend game flags + rand + key length byte
	return header.Bytes()
}

func (s *CryptState) initKey(key, challenge, data []byte) {
	for i := range data {
		challenge[(int(key[i%len(key)])*i)%len(challenge)] ^= challenge[i%len(challenge)] ^ data[i]
	}

	s.init(challenge)
}

func (s *CryptState) init(challenge []byte) {
	for i := range s.cards {
		s.cards[i] = byte(i)
	}

	var toswap, keyPos, rsum int
	for i := range slices.Backward(s.cards[:]) {
		toswap, rsum, keyPos = s.keyrand(i, challenge, rsum, keyPos)
		swaptemp := s.cards[i]
		s.cards[i] = s.cards[toswap]
		s.cards[toswap] = swaptemp
	}

	s.rotor = s.cards[1]
	s.ratchet = s.cards[3]
	s.avalanche = s.cards[5]
	s.lastPlain = s.cards[7]
	s.lastCipher = s.cards[rsum%len(s.cards)]
}

func (s *CryptState) keyrand(limit int, key []byte, rsum, keyPos int) (int, int, int) {
	if limit == 0 {
		return 0, rsum, keyPos
	}

	mask := 1
	doLoop := true
	if limit > 1 {
		for doLoop {
			mask = (mask << 1) + 1
			doLoop = mask < limit
		}
	}

	retries := 0
	u := 0
	doLoop = true
	for doLoop {
		rsum = int(s.cards[rsum%len(s.cards)] + key[keyPos])
		keyPos++

		if keyPos >= len(key) {
			keyPos = 0
			rsum += len(key)
		}

		u = rsum & mask

		retries++
		if retries > 11 {
			u %= limit
		}

		doLoop = u > limit
	}

	return u, rsum, keyPos
}

func (s *CryptState) Encrypt(data []byte) []byte {
	for i := range data {
		data[i] = s.encryptByte(data[i])
	}

	return data
}

func (s *CryptState) encryptByte(d byte) byte {
	a := s.rotor
	b := s.ratchet
	c := s.cards[a]
	s.rotor = a + 1
	s.ratchet = b + c

	a = s.lastCipher
	b = s.ratchet
	b = s.cards[b]
	c = s.cards[a]
	s.cards[a] = b

	a = s.lastPlain
	b = s.ratchet
	a = s.cards[a]
	s.cards[b] = a

	a = s.rotor
	b = s.lastPlain
	a = s.cards[a]
	s.cards[b] = a

	a = s.rotor
	s.cards[a] = c

	b = s.avalanche
	a = s.cards[c]
	c = s.lastPlain
	b = a + b
	s.avalanche = b

	a = b
	c = s.cards[c]
	b = s.ratchet
	b = s.cards[b]
	a = s.cards[a]
	c = b + c
	b = s.lastCipher
	b = s.cards[b]
	c = b + c
	b = s.cards[c]
	c = s.rotor
	c = s.cards[c]
	a = a + c
	c = s.cards[b]
	b = s.cards[a]
	c ^= b ^ d
	s.lastCipher = c
	s.lastPlain = d

	return c
}

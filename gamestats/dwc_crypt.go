package gamestats

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"owfc/common"
	"slices"
)

func makeDwcToken(gamename string, endpoint string, pid string) string {
	// base64-encoded SHA-256 because the result needs to be at least 32 chars
	h := sha256.New()
	h.Write([]byte(webSalt))
	h.Write([]byte(gamename))
	h.Write([]byte(endpoint))
	h.Write([]byte(pid))

	return base64.URLEncoding.EncodeToString(h.Sum(nil))[:32]
}

func verifyDwcHash(key string, token string, hash string) bool {
	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte(token))

	return hex.EncodeToString(h.Sum(nil)) == hash
}

func makeDwcProof(key string, data []byte) string {
	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte(base64.URLEncoding.EncodeToString(data)))
	h.Write([]byte(key))

	return hex.EncodeToString(h.Sum(nil))
}

func decryptDwc(keys common.StatsInfo, data []byte) (uint32, []byte) {
	seed := binary.BigEndian.Uint32(data[:4])
	seed ^= keys.ChecksumMask
	seed &= 0xFFFF
	seed |= seed << 16

	decoded := slices.Clone(data[4:])
	for i := range decoded {
		seed = (seed*keys.Multiplier + keys.Increment) % keys.Modulus
		decoded[i] ^= byte(seed >> 16)
	}

	// pid is part of the encryption header
	// return it and strip from the data

	return binary.LittleEndian.Uint32(decoded), decoded[4:]
}

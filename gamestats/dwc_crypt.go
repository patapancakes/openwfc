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

func decryptDwc(keys common.StatsInfo, data []byte) (uint32, []byte, bool) {
	// not encrypted
	if len(data) < 8 {
		return 0, data, false
	}

	checksum := binary.BigEndian.Uint32(data[:4])
	checksum ^= keys.ChecksumMask

	seed := checksum
	seed &= 0xFFFF
	seed |= seed << 16

	decoded := slices.Clone(data[4:])
	for i := range decoded {
		seed = (seed*keys.Multiplier + keys.Increment) % keys.Modulus
		decoded[i] ^= byte(seed >> 16)
	}

	var sum uint32
	for _, b := range decoded {
		sum += uint32(b)
	}
	if sum != checksum {
		return 0, data, false
	}

	pid := binary.LittleEndian.Uint32(decoded[:4])

	// check for new type
	if len(decoded) >= 8 && int(binary.LittleEndian.Uint32(decoded[4:])) == len(decoded[8:]) {
		return pid, decoded[8:], true
	}

	// old type
	return pid, decoded[4:], false
}

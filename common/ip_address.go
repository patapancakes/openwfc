package common

import (
	"encoding/binary"
	"math/bits"
	"net/netip"
	"strconv"
)

func IPFormatToInt(addr netip.AddrPort) (uint32, uint16) {
	return binary.BigEndian.Uint32(addr.Addr().AsSlice()), addr.Port()
}

func IPFormatToString(addr netip.AddrPort) (string, string) {
	intIP, intPort := IPFormatToInt(addr)

	return strconv.Itoa(int(int32((intIP)))), strconv.Itoa(int(intPort))
}

func IPFormatToStringLE(addr netip.AddrPort) (string, string) {
	intIP, intPort := IPFormatToInt(addr)

	// Convert to little endian and print as big endian int
	return strconv.Itoa(int(int32(bits.ReverseBytes32(intIP)))), strconv.Itoa(int(intPort))
}

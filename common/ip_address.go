package common

import (
	"encoding/binary"
	"math/bits"
	"net/netip"
	"strconv"
)

func IPFormatToInt(ip string) (uint32, uint16) {
	var addr netip.Addr
	var port uint16

	addrport, err := netip.ParseAddrPort(ip)
	if err != nil {
		addr, _ = netip.ParseAddr(ip)
	} else {
		addr = addrport.Addr()
		port = addrport.Port()
	}

	return binary.BigEndian.Uint32(addr.AsSlice()), port
}

func IPFormatToString(ip string) (string, string) {
	intIP, intPort := IPFormatToInt(ip)

	return strconv.Itoa(int(int32((intIP)))), strconv.Itoa(int(intPort))
}

func IPFormatToStringLE(ip string) (string, string) {
	intIP, intPort := IPFormatToInt(ip)

	// Convert to little endian and print as big endian int
	return strconv.Itoa(int(int32(bits.ReverseBytes32(intIP)))), strconv.Itoa(int(intPort))
}

func IPFormatBytes(ip string) []byte {
	var addr netip.Addr

	addrport, err := netip.ParseAddrPort(ip)
	if err != nil {
		addr, _ = netip.ParseAddr(ip)
	} else {
		addr = addrport.Addr()
	}

	return addr.AsSlice()
}

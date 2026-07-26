package nas

import "encoding/binary"

var invT = [16]byte{
	7, 2, 5, 10,
	11, 0, 13, 15,
	12, 1, 6, 8,
	4, 9, 3, 14,
}

var exc = [8]byte{
	1, 2, 0, 4, 3, 5, 6, 7,
}

func decodeDSUserID(userid uint64) (uint16, uint32, bool, uint8) {
	var b [8]byte

	binary.LittleEndian.PutUint64(b[:], userid)

	for i := range 6 {
		b[i] ^= 0x67
	}

	v := binary.LittleEndian.Uint64(b[:])

	v &= 1<<43 - 1
	v = (v >> 1) | ((v & 1) << 42)

	binary.LittleEndian.PutUint64(b[:], v)

	tmp := b
	for i := range 5 {
		x := tmp[exc[i]]
		b[i] = invT[x>>4]<<4 | invT[x&0x0F]
	}

	for i := range 6 {
		b[i] ^= 0xD6
	}

	v = binary.LittleEndian.Uint64(b[:])

	uid := uint16(v >> 27 & 0xFFFF)
	mac := uint32(v >> 3 & 0xFFFFFF) // last 3 octets of mac
	otherVendor := v >> 2 & 1        // if prefix not 00:09:BF
	unk := uint8(v & 0x03)           // always 0?

	return uid, mac, otherVendor == 1, unk
}

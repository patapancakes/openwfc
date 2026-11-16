package natneg

import (
	"net"
	"owfc/logging"
)

const (
	NNPreInitWaitingForClient  = 0x00
	NNPreInitWaitingForMatchup = 0x01
	NNPreInitReady             = 0x02
)

func (s *Session) handlePreinit(moduleName string, conn net.PacketConn, addr *net.UDPAddr, buffer []byte) {
	if len(buffer) < 6 {
		logging.Error(moduleName, "Invalid packet size")
		return
	}

	// clientIndex := buffer[0]
	// state := buffer[1]
	// clientID := binary.BigEndian.Uint32(buffer[2:6])

	// Not exactly sure how this is supposed to work.
	// NATNEG internally calls it "ACE queuing", but the games don't seem to take advantage of any potential "queuing" functionality.
	// Hopefully just returning "ready" will cause the games to continue with NATNEG as normal.

	packet := createPacketHeader(s.Version, NNPreInitReply, s.Cookie)
	buffer[1] = NNPreInitReady
	packet = append(packet, buffer...)
	conn.WriteTo(packet, addr)
}

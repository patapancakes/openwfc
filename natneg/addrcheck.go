package natneg

import (
	"encoding/binary"
	"net"
	"net/netip"
	"wwfc/common"
	"wwfc/logging"
)

func (session *NATNEGSession) handleAddressCheck(conn net.PacketConn, addr net.Addr, buffer []byte, moduleName string, version byte) {
	if len(buffer) < 1 {
		logging.Error(moduleName, "Invalid packet size")
		return
	}

	portType := buffer[0]
	if portType > 0x03 {
		logging.Error(moduleName, "Invalid port type")
		return
	}

	addrPort, err := netip.ParseAddrPort(addr.String())
	common.ShouldNotError(err)

	addr4 := addrPort.Addr().As4()

	response := createPacketHeader(version, NNAddressCheckReply, session.Cookie)
	response = append(response, portType)
	response = append(response, 0x00)                                   // clientIndex
	response = append(response, 0x00)                                   // useGamePort
	response = append(response, addr4[:]...)                            // client ip
	response = binary.BigEndian.AppendUint16(response, addrPort.Port()) // client port

	_, _ = conn.WriteTo(response, addr)
}

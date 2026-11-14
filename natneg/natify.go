package natneg

import (
	"encoding/binary"
	"net"
	"owfc/common"
	"owfc/logging"
)

func (session *Session) handleAddressCheck(conn net.PacketConn, addr *net.UDPAddr, buffer []byte, moduleName string, version byte) {
	if len(buffer) < 1 {
		logging.Error(moduleName, "Invalid packet size")
		return
	}

	portType := buffer[0]
	if portType > 0x03 {
		logging.Error(moduleName, "Invalid port type")
		return
	}

	response := createPacketHeader(version, NNAddressCheckReply, session.Cookie)
	response = append(response, portType)
	response = append(response, 0x00)                                     // clientIndex
	response = append(response, 0x00)                                     // useGamePort
	response = append(response, addr.IP[:]...)                            // client ip
	response = binary.BigEndian.AppendUint16(response, uint16(addr.Port)) // client port

	conn.WriteTo(response, addr)
}

func (session *Session) handleNatify(conn net.PacketConn, addr *net.UDPAddr, buffer []byte, moduleName string, version byte) {
	if len(buffer) < 1 {
		logging.Error(moduleName, "Invalid packet size")
		return
	}

	portType := buffer[0]
	if portType > 0x03 {
		logging.Error(moduleName, "Invalid port type")
		return
	}

	// NN2/NN3 different ip
	if portType == PortTypeNATNEG2 || portType == PortTypeNATNEG3 {
		config := common.GetConfig()

		srcAddr := net.UDPAddr{IP: net.ParseIP(*config.ProbeAddress), Port: 27091}
		// NN3 different port
		if portType == PortTypeNATNEG3 {
			srcAddr.Port = 0
		}

		var err error
		conn, err = net.ListenUDP("udp", &srcAddr)
		common.ShouldNotError(err)
		defer conn.Close()
	}

	response := createPacketHeader(version, NNErtTestRequest, session.Cookie)
	response = append(response, portType)
	response = append(response, make([]byte, 8)...)

	conn.WriteTo(response, addr)
}

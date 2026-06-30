package natneg

import (
	"net"
	"wwfc/common"
	"wwfc/logging"
)

func (session *NATNEGSession) handleNatify(conn net.PacketConn, addr net.Addr, buffer []byte, moduleName string, version byte) {
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

	_, _ = conn.WriteTo(response, addr)
}

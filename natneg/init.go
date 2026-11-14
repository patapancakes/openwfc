package natneg

import (
	"net"
	"owfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

func (session *Session) handleInit(conn net.PacketConn, addr *net.UDPAddr, buffer []byte, moduleName string, version byte) {
	if len(buffer) < 10 {
		logging.Error(moduleName, "Invalid packet size")
		return
	}

	portType := buffer[0]
	clientIndex := buffer[1]
	useGamePort := buffer[2] == 1
	//localIP, ok := netip.AddrFromSlice(buffer[3:7])
	//localPort := binary.BigEndian.Uint16(buffer[7:])
	//gameName, err := common.GetString(buffer[9:])

	if portType > 0x03 {
		logging.Error(moduleName, "Invalid port type")
		return
	}
	if !useGamePort && portType == PortTypeGamePort {
		logging.Error(moduleName, "Request uses game port but use game port is disabled")
		return
	}

	// Write the init acknowledgement to the requester address
	ackHeader := createPacketHeader(version, NNInitReply, session.Cookie)
	ackHeader = append(ackHeader, portType, clientIndex)
	ackHeader = append(ackHeader, 0xff, 0xff, 0x6d, 0x16, 0xb5, 0x7d, 0xea) // ?
	conn.WriteTo(ackHeader, addr)

	target := &session.Guest
	if clientIndex == 1 {
		target = &session.Host
	}
	if *target == nil {
		logging.Notice(moduleName, "Creating client index", aurora.Cyan(clientIndex))

		*target = &Peer{
			Cookie: session.Cookie,
			Host:   clientIndex == 1,
			Result: -1,
		}
	}

	client := *target
	if portType != PortTypeGamePort {
		client.NegotiateAddr = addr
	}
	if !useGamePort || portType == PortTypeGamePort {
		client.ServerAddr = addr
	}
	if !client.isMapped() {
		return
	}

	// logging.Info(moduleName, "Mapped", aurora.BrightCyan(sender.NegotiateIP), aurora.BrightCyan(sender.LocalIP), aurora.BrightCyan(sender.ServerIP))

	// Send the connect requests
	session.sendConnectRequests(moduleName)
}

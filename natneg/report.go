package natneg

import (
	"fmt"
	"net"
	"owfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

func (session *Session) handleReport(conn net.PacketConn, addr *net.UDPAddr, buffer []byte, _moduleName string, version byte) {
	if len(buffer) < 2 {
		logging.Error(_moduleName, "Invalid packet size")
		return
	}

	response := createPacketHeader(version, NNReportReply, session.Cookie)
	response = append(response, buffer[:9]...)
	response[14] = 0
	conn.WriteTo(response, addr)

	// portType := buffer[0]
	clientIndex := buffer[1]
	result := buffer[2]
	// natType := buffer[3]
	// mappingScheme := buffer[7]
	// gameName, err := common.GetString(buffer[11:])

	moduleName := "NATNEG:" + fmt.Sprintf("%08x/", session.Cookie) + addr.String()
	logging.Notice(moduleName, "Report from", aurora.BrightCyan(clientIndex), "result:", aurora.Cyan(result))

	client := session.Guest
	peer := session.Host
	if clientIndex == 1 {
		client = session.Host
		peer = session.Guest
	}
	if client == nil {
		return
	}

	client.Result = int(result)
	client.ConnectAck = false

	if peer == nil {
		return
	}

	eventType := "natneg_failed"
	if client.Result == 1 || peer.Result == 1 {
		eventType = "natneg_succeeded"
	}

	logging.Event(
		eventType,
		map[string]any{
			"peers": []string{client.ServerAddr.String(), peer.ServerAddr.String()},
		},
	)
}

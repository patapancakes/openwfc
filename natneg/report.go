package natneg

import (
	"fmt"
	"net"
	"owfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

func (s *Session) handleReport(moduleName string, conn net.PacketConn, addr *net.UDPAddr, buffer []byte) {
	if len(buffer) < 2 {
		logging.Error(moduleName, "Invalid packet size")
		return
	}

	response := createPacketHeader(s.Version, NNReportReply, s.Cookie)
	response = append(response, buffer[:9]...)
	response[14] = 0
	conn.WriteTo(response, addr)

	// portType := buffer[0]
	clientIndex := buffer[1]
	result := buffer[2]
	// natType := buffer[3]
	// mappingScheme := buffer[7]
	// gameName, err := common.GetString(buffer[11:])

	moduleName = "NATNEG:" + fmt.Sprintf("%08x/", s.Cookie) + addr.String()
	logging.Notice(moduleName, "Report from", aurora.BrightCyan(clientIndex), "result:", aurora.Cyan(result))

	client := s.Guest
	peer := s.Host
	if clientIndex == 1 {
		client = s.Host
		peer = s.Guest
	}
	if client == nil {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	client.Result = int(result)

	if peer == nil {
		return
	}

	if client.Result == -1 || peer.Result == -1 {
		// haven't gotten both reports yet
		return
	}

	eventType := "natneg_failed"
	if client.Result == 1 && peer.Result == 1 {
		eventType = "natneg_succeeded"
	}

	logging.Event(
		eventType,
		map[string]any{
			"peers": []string{client.GameAddr.String(), peer.GameAddr.String()},
		},
	)
}

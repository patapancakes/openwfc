package qr2

import (
	"net"
	"owfc/common"
	"owfc/logging"
	"strings"
)

func (s *Session) handleHeartbeat(moduleName string, conn net.PacketConn, buffer []byte) {
	values := strings.Split(string(buffer), "\x00")

	payload := map[string]string{}
	for i := 0; i < len(values)-1; i += 2 {
		if len(values[i]) == 0 {
			continue
		}

		payload[values[i]] = values[i+1]
	}

	statechanged, ok := payload["statechanged"]
	if ok && statechanged == "2" {
		logging.Notice(moduleName, "Server shut down")
		logging.Event("server_shutdown", map[string]any{
			"cookie":     s.Cookie,
			"data":       payload,
			"ip_address": s.Addr.String(),
		})

		mutex.Lock()
		removeSession(s.Cookie)
		mutex.Unlock()
		return
	}

	// DS sends these endian-flipped, if kept as-is in server list then
	// it never can verify its server is created. GS must have overwritten these...
	payload["publicip"], payload["publicport"] = common.IPFormatToString(s.Addr.String())

	s.Data = payload

	if s.Authenticated {
		// nothing to do
		return
	}

	// send challenge
	response := createResponseHeader(ChallengeRequest, s.Cookie)
	response = append(response, []byte(s.Challenge)...)
	response = append(response, 0)
	conn.WriteTo(response, s.Addr)
}

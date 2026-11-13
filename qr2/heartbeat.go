package qr2

import (
	"fmt"
	"net"
	"owfc/common"
	"owfc/logging"
	"strings"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

func heartbeat(moduleName string, conn net.PacketConn, addr *net.UDPAddr, sessionId uint32, buffer []byte) {
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
			"session_id": sessionId,
			"data":       payload,
			"ip_address": addr.String(),
		})

		mutex.Lock()
		removeSession(sessionId)
		mutex.Unlock()
		return
	}

	// DS sends these endian-flipped, if kept as-is in server list then
	// it never can verify its server is created. GS must have overwritten these...
	payload["publicip"], payload["publicport"] = common.IPFormatToString(addr.String())

	mutex.Lock()
	defer mutex.Unlock()

	session, exists := sessions[sessionId]
	if !exists {
		logging.Info(moduleName, "Creating session", aurora.Cyan(sessionId))

		session = &Session{
			SessionID: sessionId,
			Addr:      addr,
			Challenge: fmt.Sprintf("%s00%X%04X", common.RandomString(6), []byte(addr.IP), addr.Port),
		}

		sessions[sessionId] = session
	}

	session.LastKeepAlive = time.Now()
	session.Data = payload

	if session.Authenticated {
		// nothing to do
		return
	}

	// send challenge
	response := createResponseHeader(ChallengeRequest, session.SessionID)
	response = append(response, []byte(session.Challenge)...)
	response = append(response, 0)

	conn.WriteTo(response, session.Addr)
}

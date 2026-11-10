package qr2

import (
	"encoding/binary"
	"net"
	"owfc/common"
	"owfc/logging"
	"strings"
)

func heartbeat(moduleName string, conn net.PacketConn, addr net.UDPAddr, buffer []byte) {
	sessionId := binary.BigEndian.Uint32(buffer[1:5])
	values := strings.Split(string(buffer[5:]), "\u0000")

	payload := map[string]string{}
	for i := 0; i < len(values)-1; i += 2 {
		if len(values[i]) == 0 {
			continue
		}

		payload[values[i]] = values[i+1]
	}

	lookupAddr := makeLookupAddr(addr.String())

	statechanged, ok := payload["statechanged"]
	if ok && statechanged == "2" {
		logging.Notice(moduleName, "Server shut down")
		logging.Event("server_shutdown", map[string]any{
			"session_id": sessionId,
			"data":       payload,
			"ip_address": addr.String(),
		})

		mutex.Lock()
		removeSession(lookupAddr)
		mutex.Unlock()
		return
	}

	// DS sends these endian-flipped, if kept as-is in server list then
	// it never can verify its server is created. GS must have overwritten these...
	payload["publicip"], payload["publicport"] = common.IPFormatToString(addr.String())

	session, ok := setSessionData(moduleName, &addr, sessionId, payload)
	if !ok {
		return
	}

	if !session.Authenticated {
		sendChallenge(conn, addr, session, lookupAddr)
	}
}

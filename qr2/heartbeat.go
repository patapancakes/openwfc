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

	realIP, realPort := common.IPFormatToString(addr.String())

	noIP := false
	if ip, ok := payload["publicip"]; !ok || ip == "0" {
		noIP = true
	}

	clientEndianness := common.GetExpectedUnitCode(payload["gamename"])
	if !noIP && clientEndianness == ClientBigEndian {
		if payload["publicip"] != realIP || payload["publicport"] != realPort {
			// Client is mistaken about its public IP
			logging.Error(moduleName, "Public IP mismatch")
			return
		}
	} else if !noIP && clientEndianness == ClientLittleEndian {
		realIPLE, realPortLE := common.IPFormatToStringLE(addr.String())
		if payload["publicip"] != realIPLE || payload["publicport"] != realPortLE {
			// Client is mistaken about its public IP
			logging.Error(moduleName, "Public IP mismatch")
			return
		}
	}
	// Else it's a cross-compatible game and the endianness is ambiguous

	payload["publicip"] = realIP
	payload["publicport"] = realPort

	lookupAddr := makeLookupAddr(addr.String())

	statechanged, ok := payload["statechanged"]
	if ok && statechanged == "2" {
		logging.Notice(moduleName, "Client session shutdown")
		mutex.Lock()
		removeSession(lookupAddr)
		mutex.Unlock()
		return
	}

	session, ok := setSessionData(moduleName, &addr, sessionId, payload)
	if !ok {
		return
	}

	if !session.Authenticated || noIP {
		sendChallenge(conn, addr, session, lookupAddr)
	}
}

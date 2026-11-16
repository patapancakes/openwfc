package qr2

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"owfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

func SendClientMessage(dest netip.AddrPort, message []byte) {
	moduleName := "QR2/MSG"

	var receiver *Session
	mutex.RLock()
	for _, session := range sessions {
		if session.Addr.AddrPort().Compare(dest) != 0 {
			continue
		}

		receiver = session
		break
	}
	mutex.RUnlock()
	if receiver == nil || !receiver.Authenticated {
		logging.Error(moduleName, "Destination", aurora.Cyan(dest), "does not exist")
		return
	}

	// DWC-related fixes
	if bytes.HasPrefix(message, []byte("SBCM")) || bytes.HasPrefix(message, []byte{0xbb, 0x49, 0xcc, 0x4d}) {
		// DWCi_QR2ClientMsgCallback
		// message length byte isn't checked against received length
		if len(message) < 0x14 || len(message) >= 0x80 || int(message[9])+0x14 != len(message) {
			logging.Error(moduleName, "Received invalid match command packet")
			return
		}
	}

	receiver.mutex.Lock()
	msgCount := receiver.MessageCount
	receiver.MessageCount++
	receiver.mutex.Unlock()

	payload := createResponseHeader(ClientMessageRequest, receiver.Cookie)
	payload = binary.BigEndian.AppendUint32(payload, msgCount)
	payload = append(payload, message...)
	masterConn.WriteTo(payload, receiver.Addr)
}

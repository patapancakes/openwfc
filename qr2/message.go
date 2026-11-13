package qr2

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"owfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

func SendClientMessage(senderIP string, dest netip.AddrPort, message []byte) {
	moduleName := "QR2/MSG"

	mutex.Lock()

	var receiver *Session
	for _, session := range sessions {
		if session.Addr.AddrPort().Compare(dest) != 0 {
			continue
		}

		receiver = session
		break
	}
	if receiver == nil || !receiver.Authenticated {
		mutex.Unlock()
		logging.Error(moduleName, "Destination", aurora.Cyan(dest), "does not exist")
		return
	}

	mutex.Unlock()

	// DWCi_QR2ClientMsgCallback blindly trusts the size specified in the header
	// make sure it's the correct size
	if bytes.HasPrefix(message, []byte("SBCM")) || bytes.HasPrefix(message, []byte{0xbb, 0x49, 0xcc, 0x4d}) {
		// DWC match command
		if len(message) < 0x14 {
			logging.Error(moduleName, "Received invalid length match command packet")
			return
		}

		if int(message[9])+0x14 != len(message) {
			logging.Error(moduleName, "Received invalid match command packet header")
			return
		}
	}

	packetCount := receiver.PacketCount + 1
	receiver.PacketCount = packetCount

	payload := createResponseHeader(ClientMessageRequest, receiver.SessionID)

	payload = append(payload, []byte{0, 0, 0, 0}...)
	binary.BigEndian.PutUint32(payload[len(payload)-4:], packetCount)

	payload = append(payload, message...)

	masterConn.WriteTo(payload, &receiver.Addr)
}

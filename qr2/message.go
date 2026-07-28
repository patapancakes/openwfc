package qr2

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"owfc/logging"
	"time"

	"github.com/logrusorgru/aurora/v3"
	"gvisor.dev/gvisor/pkg/sleep"
)

func SendClientMessage(senderIP string, dest netip.AddrPort, message []byte) {
	moduleName := "QR2/MSG"

	mutex.Lock()

	receiver := sessions[makeLookupAddr(dest.String())]

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

	receiver.messageMutex.Lock()
	defer receiver.messageMutex.Unlock()

	var s sleep.Sleeper
	defer s.Done()

	receiver.messageAckWaker.Clear()
	s.AddWaker(receiver.messageAckWaker)

	var timeWaker sleep.Waker
	s.AddWaker(&timeWaker)

	timeOutCount := 0
	for {
		time.AfterFunc(1*time.Second, func() {
			timeWaker.Assert()
		})

		_, err := masterConn.WriteTo(payload, &receiver.Addr)
		if err != nil {
			logging.Error(moduleName, "Error sending message:", err.Error())
		}

		// Wait for an ack or timeout
		switch s.Fetch(true) {
		case &timeWaker:
			timeOutCount++

			// Enforce a 10 second timeout
			if timeOutCount <= 10 {
				break
			}

			logging.Error(moduleName, "Timed out waiting for ack")
			return

		default:
			return
		}
	}
}

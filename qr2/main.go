package qr2

import (
	"encoding/binary"
	"fmt"
	"net"
	"owfc/common"
	"owfc/database"
	"owfc/logging"
	"sync"
	"sync/atomic"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

const (
	QueryRequest            = 0x00
	ChallengeRequest        = 0x01
	EchoRequest             = 0x02
	HeartbeatRequest        = 0x03
	AddErrorRequest         = 0x04
	EchoResponseRequest     = 0x05
	ClientMessageRequest    = 0x06
	ClientMessageAckRequest = 0x07
	KeepAliveRequest        = 0x08
	AvailableRequest        = 0x09
	ClientRegisteredReply   = 0x0A
)

var (
	db         database.Connection
	masterConn net.PacketConn
	inShutdown atomic.Bool
	waitGroup  sync.WaitGroup
)

func StartServer(reload bool) {
	// Get config
	config := common.GetConfig()

	// Start SQL
	db = database.Start(config)

	address := *config.GameSpyAddress + ":27900"
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		panic(err)
	}

	masterConn = conn
	inShutdown.Store(false)

	if reload {
		err := loadSessions()
		if err != nil {
			panic(err)
		}

		logging.Notice("QR2", "Loaded", aurora.Cyan(len(sessions)), "sessions")
	}

	db.RegisterEvents(config, []string{
		"server_created",
		"server_shutdown",
	})

	waitGroup.Go(func() {
		// Close the listener when the application closes.
		defer conn.Close()
		logging.Notice("QR2", "Listening on", aurora.BrightCyan(address))

		for {
			if inShutdown.Load() {
				return
			}

			buf := make([]byte, 1024)
			n, addr, err := conn.ReadFrom(buf)
			if err != nil || n < 5 {
				continue
			}

			waitGroup.Go(func() { handleConnection(conn, addr.(*net.UDPAddr), buf[:n]) })
		}
	})
}

func Shutdown() {
	inShutdown.Store(true)
	common.ShouldNotError(masterConn.Close())
	waitGroup.Wait()

	mutex.Lock()
	defer mutex.Unlock()

	err := saveSessions()
	if err != nil {
		logging.Error("QR2", "Failed to save sessions:", err)
	}

	db.Close()

	logging.Notice("QR2", "Saved", aurora.Cyan(len(sessions)), "sessions")
}

func handleConnection(conn net.PacketConn, addr *net.UDPAddr, buffer []byte) {
	packetType := buffer[0]
	cookie := binary.BigEndian.Uint32(buffer[1:])
	moduleName := "QR2:" + addr.String()

	var session *Session
	if packetType != AvailableRequest {
		var exists bool

		mutex.RLock()
		session, exists = sessions[cookie]
		mutex.RUnlock()
		if !exists {
			logging.Info(moduleName, "Creating session", aurora.Cyan(cookie))

			session = &Session{
				Cookie:        cookie,
				Addr:          addr,
				Challenge:     fmt.Sprintf("%s00%X%04X", common.RandomString(6), []byte(addr.IP), addr.Port),
				LastKeepAlive: time.Now(),
			}

			mutex.Lock()
			sessions[cookie] = session
			mutex.Unlock()
		}
	}

	switch packetType {
	case ChallengeRequest:
		logging.Info(moduleName, "Command:", aurora.Yellow("CHALLENGE"))
		session.handleChallenge(moduleName, conn, buffer[5:])

	case HeartbeatRequest:
		// logging.Info(moduleName, "Command:", aurora.Yellow("HEARTBEAT"))
		session.handleHeartbeat(moduleName, conn, buffer[5:])

	case ClientMessageAckRequest:
		// logging.Info(moduleName, "Command:", aurora.Yellow("CLIENT_MESSAGE_ACK"))
		// TODO: verify these

	case KeepAliveRequest:
		// logging.Info(moduleName, "Command:", aurora.Yellow("KEEPALIVE"))
		session.LastKeepAlive = time.Now()

		conn.WriteTo(createResponseHeader(KeepAliveRequest, cookie), addr)

	case AvailableRequest:
		logging.Info("QR2", "Command:", aurora.Yellow("AVAILABLE"))

		gameName, err := common.GetString(buffer[5:])
		if err != nil {
			return
		}

		// Only respond if the game name provided in the packet is valid
		_, exists := common.GetGameInfoByName(gameName)
		if !exists {
			return
		}

		conn.WriteTo(createResponseHeader(AvailableRequest, cookie), addr)

	default:
		logging.Error(moduleName, "Unknown command:", aurora.Yellow(buffer[0]))
	}
}

func createResponseHeader(command byte, cookie uint32) []byte {
	return binary.BigEndian.AppendUint32([]byte{0xfe, 0xfd, command}, cookie)
}

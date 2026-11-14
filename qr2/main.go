package qr2

import (
	"encoding/binary"
	"fmt"
	"net"
	"owfc/common"
	"owfc/database"
	"owfc/logging"
	"strings"
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
	sessionId := binary.BigEndian.Uint32(buffer[1:])
	moduleName := "QR2:" + addr.String()

	var session *Session
	if packetType != HeartbeatRequest && packetType != AvailableRequest {
		mutex.Lock()

		var ok bool
		session, ok = sessions[sessionId]
		if !ok {
			mutex.Unlock()
			logging.Error(moduleName, "Cannot find session for this IP address")
			return
		}

		session.SessionID = sessionId
		mutex.Unlock()
	}

	switch packetType {
	case ChallengeRequest:
		logging.Info(moduleName, "Command:", aurora.Yellow("CHALLENGE"))

		mutex.Lock()
		defer mutex.Unlock()

		game, ok := common.GetGameInfoByName(session.Data["gamename"])
		if !ok {
			logging.Error(moduleName, "Unknown game:", aurora.Cyan(session.Data["gamename"]))
			return
		}

		challenge, err := common.GetString(buffer[5:])
		if err != nil {
			logging.Error(moduleName, "Invalid challenge")
			return
		}

		if challenge != encode(encrypt([]byte(game.SecretKey), []byte(session.Challenge))) {
			logging.Error(moduleName, "Invalid challenge")
			return
		}

		session.Authenticated = true

		var data strings.Builder
		for k, v := range session.Data {
			if k == "gamename" || k == "statechanged" {
				continue
			}

			if data.Len() != 0 {
				data.WriteString(", ")
			}

			fmt.Fprintf(&data, "%s: %s", k, v)
		}

		logging.Notice(moduleName, "Server created:", aurora.Cyan(game.Name), "/", aurora.Cyan(&data))
		logging.Event("server_created", map[string]any{
			"session_id": session.SessionID,
			"data":       session.Data,
			"ip_address": addr.String(),
		})

		conn.WriteTo(createResponseHeader(ClientRegisteredReply, session.SessionID), addr)

	case HeartbeatRequest:
		// logging.Info(moduleName, "Command:", aurora.Yellow("HEARTBEAT"))
		heartbeat(moduleName, conn, addr, sessionId, buffer[5:])

	case ClientMessageAckRequest:
		// logging.Info(moduleName, "Command:", aurora.Yellow("CLIENT_MESSAGE_ACK"))

	case KeepAliveRequest:
		// logging.Info(moduleName, "Command:", aurora.Yellow("KEEPALIVE"))
		session.LastKeepAlive = time.Now()

		conn.WriteTo(createResponseHeader(KeepAliveRequest, 0), addr)

	case AvailableRequest:
		gameName, err := common.GetString(buffer[5:])
		if err != nil {
			return
		}

		// Only respond if the game name provided in the packet is valid
		_, exists := common.GetGameInfoByName(gameName)
		if !exists {
			return
		}

		logging.Info("QR2", "Command:", aurora.Yellow("AVAILABLE"))
		conn.WriteTo(createResponseHeader(AvailableRequest, 0), addr)

	default:
		logging.Error(moduleName, "Unknown command:", aurora.Yellow(buffer[0]))
	}
}

func createResponseHeader(command byte, sessionId uint32) []byte {
	return binary.BigEndian.AppendUint32([]byte{0xfe, 0xfd, command}, sessionId)
}

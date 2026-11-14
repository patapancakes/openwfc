package natneg

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"owfc/common"
	"owfc/database"
	"owfc/logging"
	"sync"
	"sync/atomic"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

const (
	NNInitRequest         = 0x00
	NNInitReply           = 0x01
	NNErtTestRequest      = 0x02
	NNErtTestReply        = 0x03
	NNStateUpdate         = 0x04
	NNConnectRequest      = 0x05
	NNConnectReply        = 0x06
	NNConnectPing         = 0x07
	NNBackupTestRequest   = 0x08
	NNBackupTestReply     = 0x09
	NNAddressCheckRequest = 0x0A
	NNAddressCheckReply   = 0x0B
	NNNatifyRequest       = 0x0C
	NNReportRequest       = 0x0D
	NNReportReply         = 0x0E
	NNPreInitRequest      = 0x0F
	NNPreInitReply        = 0x10

	// Port type
	PortTypeGamePort = 0x00
	PortTypeNATNEG1  = 0x01
	PortTypeNATNEG2  = 0x02
	PortTypeNATNEG3  = 0x03

	// NAT type
	NATTypeNoNat              = 0x00
	NATTypeFirewallOnly       = 0x01
	NATTypeFullCone           = 0x02
	NATTypeRestrictedCone     = 0x03
	NATTypePortRestrictedCone = 0x04
	NATTypeSymmetric          = 0x05
	NATTypeUnknown            = 0x06

	// NAT mapping scheme
	NATMappingUnknown           = 0x00
	NATMappingSamePrivatePublic = 0x01
	NATMappingConsistent        = 0x02
	NATMappingIncremental       = 0x03
	NATMappingMixed             = 0x04
)

var Magic = []byte{0xfd, 0xfc, 0x1e, 0x66, 0x6a, 0xb2}

type Session struct {
	Open        bool
	Version     byte
	Cookie      uint32
	mutex       sync.RWMutex
	Connecting  bool
	Host, Guest *Peer
}

type Peer struct {
	Cookie        uint32
	Host          bool
	ConnectAck    bool
	Result        int
	NegotiateAddr *net.UDPAddr
	ServerAddr    *net.UDPAddr
}

var (
	db database.Connection

	sessions   = map[uint32]*Session{}
	mutex      sync.RWMutex
	natnegConn net.PacketConn

	inShutdown atomic.Bool
	waitGroup  sync.WaitGroup
)

func StartServer(reload bool) {
	// Get config
	config := common.GetConfig()

	address := *config.GameSpyAddress + ":27901"
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		panic(err)
	}

	db = database.Start(config)

	// Connect to database for event logging
	if config.EventReporting.LogToDatabase {
		db.RegisterEvents(config, []string{
			"natneg_succeeded",
			"natneg_failed",
		})
	}

	natnegConn = conn
	inShutdown.Store(false)

	if reload {
		// Load state
		file, err := os.Open("state/natneg_sessions.gob")
		if err != nil {
			panic(err)
		}
		defer func() {
			common.ShouldNotError(file.Close())
		}()

		decoder := gob.NewDecoder(file)
		common.ShouldNotError(decoder.Decode(&sessions))

		for _, session := range sessions {
			cur := session
			time.AfterFunc(30*time.Second, func() {
				closeSession("NATNEG:"+fmt.Sprintf("%08x", cur.Cookie), cur)
			})
		}

		logging.Notice("NATNEG", "Loaded", aurora.Cyan(len(sessions)), "sessions")
	}

	waitGroup.Go(func() {
		// Close the listener when the application closes.
		defer conn.Close()
		logging.Notice("NATNEG", "Listening on", aurora.BrightCyan(address))

		for {
			if inShutdown.Load() {
				return
			}

			buf := make([]byte, 1024)
			n, addr, err := conn.ReadFrom(buf)
			if err != nil || n < 12 {
				continue
			}

			msg, hasPrefix := bytes.CutPrefix(buf[:n], Magic)
			if !hasPrefix {
				continue
			}

			waitGroup.Go(func() { handleConnection(conn, addr.(*net.UDPAddr), msg) })
		}
	})
}

func Shutdown() {
	inShutdown.Store(true)
	common.ShouldNotError(natnegConn.Close())
	waitGroup.Wait()

	// Save state
	mutex.Lock()
	defer mutex.Unlock()

	file, err := os.OpenFile("state/natneg_sessions.gob", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer func() {
		common.ShouldNotError(file.Close())
	}()

	db.Close()

	encoder := gob.NewEncoder(file)
	common.ShouldNotError(encoder.Encode(sessions))

	logging.Notice("NATNEG", "Saved", aurora.Cyan(len(sessions)), "sessions")
}

func handleConnection(conn net.PacketConn, addr *net.UDPAddr, buffer []byte) {
	// Parse the NATNEG header
	// fd fc 1e 66 6a b2 - Packet Magic
	// xx                - Version
	// xx                - Packet Type / Command
	// xx xx xx xx       - Cookie

	version := buffer[0]
	command := buffer[1]
	cookie := binary.BigEndian.Uint32(buffer[2:])

	moduleName := "NATNEG:" + fmt.Sprintf("%08x/", cookie) + addr.String()

	mutex.Lock()
	session, exists := sessions[cookie]
	if !exists {
		logging.Info(moduleName, "Creating session")
		session = &Session{
			Open:    true,
			Version: version,
			Cookie:  cookie,
			mutex:   sync.RWMutex{},
		}
		sessions[cookie] = session

		// Session has TTL of 30 seconds
		time.AfterFunc(30*time.Second, func() {
			closeSession(moduleName, session)
		})
	}
	mutex.Unlock()

	session.mutex.Lock()
	defer session.mutex.Unlock()

	switch command {
	default:
		logging.Error(moduleName, "Received unknown command type:", aurora.Cyan(command))

	case NNInitRequest:
		// logging.Info(moduleName, "Command:", aurora.Yellow("NN_INIT"))
		session.handleInit(conn, addr, buffer[6:], moduleName, version)

	case NNErtTestReply:
		logging.Info(moduleName, "Command:", aurora.Yellow("NN_ERTACK"))

	case NNStateUpdate:
		logging.Info(moduleName, "Command:", aurora.Yellow("NN_STATEUPDATE"))

	case NNConnectReply:
		// logging.Info(moduleName, "Command:", aurora.Yellow("NN_CONNECT_ACK"))
		session.handleConnectReply(conn, addr, buffer[6:], moduleName, version)

	case NNConnectPing:
		logging.Info(moduleName, "Command:", aurora.Yellow("NN_CONNECT_PING"))

	case NNBackupTestRequest:
		logging.Info(moduleName, "Command:", aurora.Yellow("NN_BACKUP_TEST"))

	case NNAddressCheckRequest:
		//logging.Info(moduleName, "Command:", aurora.Yellow("NN_ADDRESS_CHECK"))
		session.handleAddressCheck(conn, addr, buffer[6:], moduleName, version)

	case NNNatifyRequest:
		//logging.Info(moduleName, "Command:", aurora.Yellow("NN_NATIFY_REQUEST"))
		session.handleNatify(conn, addr, buffer[6:], moduleName, version)

	case NNReportRequest:
		// logging.Info(moduleName, "Command:", aurora.Yellow("NN_REPORT"))
		session.handleReport(conn, addr, buffer[6:], moduleName, version)

	case NNPreInitRequest:
		logging.Info(moduleName, "Command:", aurora.Yellow("NN_PREINIT"))
		session.handlePreinit(conn, addr, buffer[6:], moduleName, version)
	}
}

func closeSession(moduleName string, session *Session) {
	mutex.Lock()
	if inShutdown.Load() {
		mutex.Unlock()
		return
	}

	session.Open = false
	delete(sessions, session.Cookie)
	mutex.Unlock()

	session.mutex.Lock()
	defer session.mutex.Unlock()

	// Disconnect each client
	for _, client := range []*Peer{session.Host, session.Guest} {
		if client == nil {
			continue
		}

		index := 0
		if client.Host {
			index = 1
		}

		logging.Info(moduleName, "Disconnecting client", aurora.Cyan(index))
		// Send report ack, which will cause the client to cancel
		reportAck := createPacketHeader(session.Version, NNReportReply, session.Cookie)
		reportAck = append(reportAck, 0x00, byte(index), 0x00)
		reportAck = append(reportAck, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00)

		natnegConn.WriteTo(reportAck, client.NegotiateAddr)
	}

	logging.Info(moduleName, "Deleted session")
}

func (client *Peer) isMapped() bool {
	return client.NegotiateAddr != nil && client.ServerAddr != nil
}

func createPacketHeader(version byte, command byte, cookie uint32) []byte {
	header := append(Magic, version, command)
	return binary.BigEndian.AppendUint32(header, cookie)
}

package qr2

import (
	"encoding/gob"
	"net"
	"os"
	"owfc/common"
	"owfc/logging"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/logrusorgru/aurora/v3"
	"gvisor.dev/gvisor/pkg/sleep"
)

const (
	ClientLittleEndian = iota
	ClientBigEndian
	ClientNoEndian
)

type Session struct {
	SessionID       uint32
	Addr            net.UDPAddr
	Challenge       string
	Authenticated   bool
	LastKeepAlive   int64
	Endianness      byte // Some fields depend on the client's endianness
	Data            map[string]string
	PacketCount     uint32
	messageMutex    *deadlock.Mutex
	messageAckWaker *sleep.Waker
}

var (
	sessions = map[uint64]*Session{}
	mutex    = deadlock.Mutex{}
)

// Remove a session. Expects the global mutex to already be locked.
func removeSession(addr uint64) {
	session := sessions[addr]
	if session == nil {
		return
	}

	session.messageAckWaker.Assert()

	delete(sessions, addr)
}

// Update session data, creating the session if it doesn't exist. Returns a copy of the session data.
func setSessionData(moduleName string, addr net.Addr, sessionId uint32, payload map[string]string) (Session, bool) {
	lookupAddr := makeLookupAddr(addr.String())

	// Moving into performing operations on the session data, so lock the mutex
	mutex.Lock()
	defer mutex.Unlock()
	session, sessionExists := sessions[lookupAddr]

	if sessionExists && session.Addr.String() != addr.String() {
		logging.Error(moduleName, "Session IP mismatch")
		return Session{}, false
	}

	if !sessionExists {
		session = &Session{
			SessionID:       sessionId,
			Addr:            *addr.(*net.UDPAddr),
			Challenge:       "",
			Authenticated:   false,
			LastKeepAlive:   time.Now().UTC().Unix(),
			Endianness:      ClientNoEndian,
			Data:            payload,
			PacketCount:     0,
			messageMutex:    &deadlock.Mutex{},
			messageAckWaker: &sleep.Waker{},
		}
	}

	if !sessionExists {
		logging.Info(moduleName, "Creating session", aurora.Cyan(sessionId).String())

		sessions[lookupAddr] = session
		return *session, true
	}

	session.Data = payload
	session.LastKeepAlive = time.Now().UTC().Unix()
	session.SessionID = sessionId
	return *session, true
}

func makeLookupAddr(addr string) uint64 {
	ip, port := common.IPFormatToInt(addr)
	return (uint64(port) << 32) | uint64(ip)
}

// Get a copy of the list of servers
func GetSessionServers() []map[string]string {
	var servers []map[string]string
	var unreachable []uint64
	currentTime := time.Now().UTC().Unix()

	mutex.Lock()
	defer mutex.Unlock()
	for sessionAddr, session := range sessions {
		// If the last keep alive was over a minute ago then consider the server unreachable
		if session.LastKeepAlive < currentTime-60 {
			// If the last keep alive was over an hour ago then remove the server
			if session.LastKeepAlive < currentTime-((60*60)*1) {
				unreachable = append(unreachable, sessionAddr)
			}
			continue
		}

		if !session.Authenticated {
			continue
		}

		servers = append(servers, session.Data)
	}

	// Remove unreachable sessions
	for _, sessionAddr := range unreachable {
		logging.Notice("QR2", "Removing unreachable session", aurora.BrightCyan(sessions[sessionAddr].Addr.String()))
		removeSession(sessionAddr)
	}

	return servers
}

// Save the sessions to a file. Expects the mutex to be locked.
func saveSessions() error {
	file, err := os.OpenFile("state/qr2_sessions.gob", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		common.ShouldNotError(file.Close())
	}()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(sessions)
}

// Load the sessions from a file. Expects the mutex to be locked.
func loadSessions() error {
	file, err := os.Open("state/qr2_sessions.gob")
	if err != nil {
		return err
	}
	defer func() {
		common.ShouldNotError(file.Close())
	}()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&sessions)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		session.messageMutex = &deadlock.Mutex{}
		session.messageAckWaker = &sleep.Waker{}
	}

	return nil
}

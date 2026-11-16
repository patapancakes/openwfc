package qr2

import (
	"encoding/gob"
	"net"
	"os"
	"owfc/common"
	"owfc/logging"
	"sync"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

const (
	ClientLittleEndian = iota
	ClientBigEndian
	ClientNoEndian
)

type Session struct {
	Cookie        uint32
	Addr          *net.UDPAddr
	Challenge     string
	Authenticated bool
	LastKeepAlive time.Time
	Data          map[string]string
	MessageCount  uint32

	mutex sync.Mutex
}

var (
	sessions = map[uint32]*Session{}
	mutex    sync.RWMutex
)

// Remove a session. Expects the global mutex to already be locked.
func removeSession(cookie uint32) {
	session := sessions[cookie]
	if session == nil {
		return
	}

	delete(sessions, cookie)
}

// Get a copy of the list of servers
func GetSessionServers() []map[string]string {
	var servers []map[string]string
	var unreachable []uint32

	mutex.RLock()
	for cookie, session := range sessions {
		// If the last keep alive was over a minute ago then consider the server unreachable
		if time.Since(session.LastKeepAlive) > time.Minute {
			unreachable = append(unreachable, cookie)
			continue
		}
		if !session.Authenticated {
			continue
		}

		servers = append(servers, session.Data)
	}
	mutex.RUnlock()

	// Remove unreachable sessions
	if len(unreachable) != 0 {
		mutex.Lock()
		for _, cookie := range unreachable {
			logging.Notice("QR2", "Removing unreachable session", aurora.BrightCyan(sessions[cookie].Addr))
			removeSession(cookie)
		}
		mutex.Unlock()
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

	return nil
}

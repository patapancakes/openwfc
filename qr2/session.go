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
)

const (
	ClientLittleEndian = iota
	ClientBigEndian
	ClientNoEndian
)

type Session struct {
	SessionID     uint32
	Addr          *net.UDPAddr
	Challenge     string
	Authenticated bool
	LastKeepAlive time.Time
	Data          map[string]string
	PacketCount   uint32
}

var (
	sessions = map[uint32]*Session{}
	mutex    = deadlock.Mutex{}
)

// Remove a session. Expects the global mutex to already be locked.
func removeSession(id uint32) {
	session := sessions[id]
	if session == nil {
		return
	}

	delete(sessions, id)
}

// Get a copy of the list of servers
func GetSessionServers() []map[string]string {
	var servers []map[string]string
	var unreachable []uint32

	mutex.Lock()
	defer mutex.Unlock()
	for sessionAddr, session := range sessions {
		// If the last keep alive was over a minute ago then consider the server unreachable
		if time.Since(session.LastKeepAlive) > time.Minute {
			// If the last keep alive was over an hour ago then remove the server
			if time.Since(session.LastKeepAlive) > time.Hour {
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
	for _, id := range unreachable {
		logging.Notice("QR2", "Removing unreachable session", aurora.BrightCyan(sessions[id].Addr))
		removeSession(id)
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

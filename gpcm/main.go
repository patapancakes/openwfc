package gpcm

import (
	"bytes"
	"encoding/gob"
	"errors"
	"os"
	"owfc/common"
	"owfc/common/gamespy"
	"owfc/database"
	"owfc/logging"
	"strings"

	"github.com/linkdata/deadlock"
	"github.com/logrusorgru/aurora/v3"
)

const ServerName = "gpcm"

type GameSpySession struct {
	ConnIndex   uint64
	RemoteAddr  string
	Profile     database.Profile
	ModuleName  string
	LoggedIn    bool
	Challenge   string
	AuthToken   string
	LoginTicket string
	SessionKey  int32

	LoginInfoSet      bool
	GameName          string
	GameCode          string
	Region            byte
	Language          byte
	InGameName        string
	ConsoleFriendCode uint64
	HostPlatform      string
	UnitCode          byte

	Status    string
	LocString string

	ReadBuffer  []byte
	WriteBuffer string
}

var (
	db database.Connection

	// I would use a sync.Map instead of the map mutex combo, but this performs better.
	sessions            = map[uint32]*GameSpySession{}
	sessionsByConnIndex = map[uint64]*GameSpySession{}
	mutex               = deadlock.Mutex{}

	handlers = gamespy.Router{
		"ka":    gamespy.Handle(KeepAlive),
		"login": gamespy.Handle(login),

		"updatepro":  gamespy.Handle(updateProfile),
		"status":     gamespy.Handle(status),
		"authadd":    gamespy.Handle(authAdd),
		"addbuddy":   gamespy.Handle(addBuddy),
		"delbuddy":   gamespy.Handle(delBuddy),
		"bm":         gamespy.Handle(buddyMessage),
		"getprofile": gamespy.Handle(getProfile),
	}
)

func StartServer(reload bool) {
	// Get config
	config := common.GetConfig()

	// Start SQL
	db = database.Start(config)

	if reload {
		err := loadState()
		if err != nil {
			logging.Error("GPCM", "Failed to load state:", err)
			os.Exit(1)
		}

		logging.Notice("GPCM", "Loaded", aurora.Cyan(len(sessions)), "sessions")
	}

	db.RegisterEvents(config, []string{
		"profile_created",
		"logged_in",
		"logged_out",
		"received_login_info",
		"gpcm_returned_error",
	})
}

func Shutdown() {
	err := saveState()
	if err != nil {
		logging.Error("GPCM", "Failed to save state:", err)
	}

	db.Close()

	logging.Notice("GPCM", "Saved", aurora.Cyan(len(sessions)), "sessions")
}

func CloseConnection(index uint64) {
	mutex.Lock()
	session := sessionsByConnIndex[index]
	mutex.Unlock()

	if session == nil {
		logging.Error("GPCM", "Cannot find session for this connection index:", aurora.Cyan(index))
		return
	}

	logging.Notice(session.ModuleName, "Connection closed")

	if session.LoggedIn {
		session.sendLogoutStatus()

		logging.Event("logged_out", map[string]any{
			"profile_id": session.Profile.ID,
		})
	}

	mutex.Lock()
	defer mutex.Unlock()

	if session.LoggedIn {
		session.LoggedIn = false
		delete(sessions, session.Profile.ID)
	}
}

type ChallengeRequest struct {
	Command   int    `gs:"lc"`
	Challenge string `gs:"challenge"`
	ID        int    `gs:"id"`
}

func NewConnection(index uint64, address string) {
	session := &GameSpySession{
		ConnIndex:  index,
		RemoteAddr: address,
		Profile:    database.Profile{},
		ModuleName: "GPCM:" + address,
		Challenge:  common.RandomString(10),
	}

	err := common.SendPacket(ServerName, index, []byte(gamespy.Marshal(ChallengeRequest{
		Command:   1,
		Challenge: session.Challenge,
		ID:        1,
	})))
	if err != nil {
		logging.Error("GPCM", "Failed to send login challenge packet:", err)
		_ = common.CloseConnection(ServerName, index)
		return
	}

	logging.Notice(session.ModuleName, "Connection established from", address)

	mutex.Lock()
	sessionsByConnIndex[index] = session
	mutex.Unlock()
}

func HandlePacket(index uint64, data []byte) {
	mutex.Lock()
	session := sessionsByConnIndex[index]
	mutex.Unlock()

	if session == nil {
		logging.Error("GPCM", "Cannot find session for this connection index:", aurora.Cyan(index))
		_ = common.CloseConnection(ServerName, index)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			logging.Error(session.ModuleName, "Panic:", r)
		}
	}()

	// Enforce maximum buffer size
	length := len(session.ReadBuffer) + len(data)
	if length > 0x4000 {
		logging.Error(session.ModuleName, "Buffer overflow")
		return
	}

	session.ReadBuffer = append(session.ReadBuffer, data...)

	// Packets can be received in fragments, so make sure we're at the end of a packet
	if !bytes.HasSuffix(session.ReadBuffer, []byte(gamespy.EndDelimiter)) {
		return
	}

	for _, message := range strings.SplitAfter(string(session.ReadBuffer), gamespy.EndDelimiter) {
		if len(message) == 0 {
			continue
		}

		command, _, _ := strings.Cut(strings.TrimPrefix(message, `\`), `\`)
		handler, ok := handlers[command]
		if !ok {
			logging.Error(session.ModuleName, "Unknown command:", aurora.Cyan(command))
			continue
		}

		resp, err := handler(session, message)
		if err != nil {
			gpErr, ok := errors.AsType[GPError](err)
			if ok {
				session.replyError(gpErr)
				// TODO: return now on fatal?
			}

			// TODO: log this
			continue
		}

		session.WriteBuffer += resp

		// HACK: send friends info after login
		if command == "login" {
			session.flushBuffer()
			session.sendFriendsInfo()
		}
	}

	session.ReadBuffer = []byte{}
	session.flushBuffer()
}

func (g *GameSpySession) flushBuffer() {
	if g.WriteBuffer == "" {
		return
	}

	err := common.SendPacket(ServerName, g.ConnIndex, []byte(g.WriteBuffer))
	if err != nil {
		logging.Error(g.ModuleName, "Failed to send response packet:", err)
		return
	}

	g.WriteBuffer = ""
}

func saveState() error {
	file, err := os.OpenFile("state/gpcm_sessions.gob", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	encoder := gob.NewEncoder(file)

	mutex.Lock()
	defer mutex.Unlock()

	err = encoder.Encode(sessions)
	common.ShouldNotError(file.Close())
	return err
}

func loadState() error {
	file, err := os.Open("state/gpcm_sessions.gob")
	if err != nil {
		return err
	}

	decoder := gob.NewDecoder(file)

	mutex.Lock()
	defer mutex.Unlock()

	err = decoder.Decode(&sessions)
	common.ShouldNotError(file.Close())
	if err != nil {
		return err
	}

	for _, session := range sessions {
		sessionsByConnIndex[session.ConnIndex] = session
	}

	return nil
}

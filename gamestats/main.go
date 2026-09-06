package gamestats

import (
	"bytes"
	"encoding/gob"
	"errors"
	"os"
	"owfc/common"
	"owfc/common/gamespy"
	"owfc/database"
	"owfc/gpcm"
	"owfc/logging"
	"strings"

	"github.com/linkdata/deadlock"
	"github.com/logrusorgru/aurora/v3"
)

var ServerName = "gamestats"

type GameStatsSession struct {
	ConnIndex  uint64
	RemoteAddr string
	ModuleName string
	Challenge  string

	SessionKey int32
	GameName   string
	gameInfo   common.GameInfo

	Authenticated bool
	LocalID       int
	Profile       database.Profile

	ReadBuffer  []byte
	WriteBuffer []byte
}

var (
	db database.Connection

	serverName string
	webSalt    string

	sessionsByConnIndex = make(map[uint64]*GameStatsSession)
	mutex               = deadlock.RWMutex{}

	handlers = gamespy.Router{
		"ka": gamespy.Handle(gpcm.KeepAlive),

		"auth":  gamespy.Handle(auth),
		"authp": gamespy.Handle(authProfile),

		"getpd": gamespy.Handle(getPersistData),
		"setpd": gamespy.Handle(setPersistData),
	}
)

const (
	PrivateRead = iota
	PrivateReadWrite
	PublicRead
	PublicReadWrite
)

func StartServer(reload bool) {
	// Get config
	config := common.GetConfig()

	serverName = config.ServerName
	webSalt = common.RandomString(32)

	common.ReadGameList()

	// Start SQL
	db = database.Start(config)

	if reload {
		// Load state
		file, err := os.Open("state/gstats_sessions.gob")
		if err != nil {
			panic(err)
		}
		defer func() {
			common.ShouldNotError(file.Close())
		}()

		decoder := gob.NewDecoder(file)
		common.ShouldNotError(decoder.Decode(&sessionsByConnIndex))

		for _, session := range sessionsByConnIndex {
			var ok bool
			session.gameInfo, ok = common.GetGameInfoByName(session.GameName)
			if !ok {
				logging.Error(session.ModuleName, "Unknown game from reload:", aurora.Cyan(session.GameName))
				// Force close the session now to prevent a panic later
				_ = common.CloseConnection(ServerName, session.ConnIndex)
				delete(sessionsByConnIndex, session.ConnIndex)
			}
		}

		logging.Notice("GSTATS", "Loaded", aurora.Cyan(len(sessionsByConnIndex)), "sessions")
	}
}

func Shutdown() {
	// Save state
	file, err := os.OpenFile("state/gstats_sessions.gob", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	common.ShouldNotError(err)
	defer func() {
		common.ShouldNotError(file.Close())
	}()
	defer db.Close()

	encoder := gob.NewEncoder(file)
	common.ShouldNotError(encoder.Encode(sessionsByConnIndex))

	logging.Notice("GSTATS", "Saved", aurora.Cyan(len(sessionsByConnIndex)), "sessions")
}

func NewConnection(index uint64, address string) {
	session := &GameStatsSession{
		ConnIndex:  index,
		RemoteAddr: address,
		ModuleName: "GSTATS:" + address,
		Challenge:  common.RandomString(10),
	}

	session.write(gamespy.Marshal(gpcm.ChallengeRequest{
		Command:   1,
		Challenge: session.Challenge,
		ID:        1,
	}))
	err := common.SendPacket(ServerName, index, []byte(session.WriteBuffer))
	if err != nil {
		logging.Error(session.ModuleName, "Failed to send initial packet:", err)
	}
	session.WriteBuffer = []byte{}

	logging.Notice(session.ModuleName, "Connection established from", address)

	mutex.Lock()
	sessionsByConnIndex[index] = session
	mutex.Unlock()
}

func CloseConnection(index uint64) {
	mutex.RLock()
	session := sessionsByConnIndex[index]
	mutex.RUnlock()

	if session == nil {
		logging.Error("GSTATS", "Cannot find session for this connection index:", aurora.Cyan(index))
		return
	}

	logging.Notice(session.ModuleName, "Connection closed")

	mutex.Lock()
	delete(sessionsByConnIndex, index)
	mutex.Unlock()
}

func HandlePacket(index uint64, data []byte) {
	mutex.RLock()
	session := sessionsByConnIndex[index]
	mutex.RUnlock()

	if session == nil {
		logging.Error("GSTATS", "Cannot find session for this connection index:", aurora.Cyan(index))
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

		message = read(message)

		command, _, _ := strings.Cut(strings.TrimPrefix(message, `\`), `\`)
		handler, ok := handlers[command]
		if !ok {
			logging.Error(session.ModuleName, "Unknown command:", aurora.Cyan(command))
			continue
		}

		resp, err := handler(session, message)
		if err != nil {
			gpErr, ok := errors.AsType[gpcm.GPError](err)
			if ok {
				session.replyError(gpErr)
				// TODO: return now on fatal?
			}

			// TODO: log this
			continue
		}

		session.write(resp)
	}

	session.ReadBuffer = []byte{}

	if len(session.WriteBuffer) == 0 {
		return
	}

	err := common.SendPacket(ServerName, session.ConnIndex, session.WriteBuffer)
	if err != nil {
		logging.Error(session.ModuleName, "Failed to send packet:", err)
		return
	}

	session.WriteBuffer = []byte{}
}

func (g *GameStatsSession) write(msg string) {
	g.WriteBuffer = append(g.WriteBuffer, crypt(strings.TrimSuffix(msg, gamespy.EndDelimiter))+gamespy.EndDelimiter...)
}

func read(s string) string {
	return crypt(strings.TrimSuffix(s, gamespy.EndDelimiter)) + gamespy.EndDelimiter
}

func crypt(s string) string {
	const key = "GameSpy3D"

	var crypted strings.Builder
	for i, r := range s {
		crypted.WriteByte(byte(r) ^ key[i%len(key)])
	}

	return crypted.String()
}

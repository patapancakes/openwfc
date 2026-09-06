package gpsp

import (
	"errors"
	"owfc/common"
	"owfc/common/gamespy"
	"owfc/database"
	"owfc/gpcm"
	"owfc/logging"
	"strings"
)

var ServerName = "gpsp"

var (
	db database.Connection

	handlers = gamespy.Router{
		"ka": gamespy.Handle(gpcm.KeepAlive),

		"otherslist": gamespy.Handle(othersList),
		"search":     gamespy.Handle(search),
	}
)

func StartServer(reload bool) {
	// Get config
	config := common.GetConfig()

	// Start SQL
	db = database.Start(config)
}

func Shutdown() {
	db.Close()
}

func NewConnection(index uint64, address string) {
}

func CloseConnection(index uint64) {
}

func HandlePacket(index uint64, data []byte) {
	moduleName := "GPSP"

	for _, message := range strings.SplitAfter(string(data), gamespy.EndDelimiter) {
		if len(message) == 0 {
			continue
		}

		command, _, _ := strings.Cut(strings.TrimPrefix(message, `\`), `\`)
		handler, ok := handlers[command]
		if !ok {
			logging.Error(moduleName, "Unknown command:", command)
			replyError(moduleName, index, gpcm.ErrParse)
		}

		resp, err := handler(nil, message)
		if err != nil {
			gpErr, ok := errors.AsType[gpcm.GPError](err)
			if ok {
				replyError(moduleName, index, gpErr)
				// TODO: return now on fatal?
			}

			// TODO: log this
			continue
		}

		err = common.SendPacket(ServerName, index, []byte(resp))
		if err != nil {
			logging.Error(moduleName, "Failed to send packet:", err)
		}
	}
}

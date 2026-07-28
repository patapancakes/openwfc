package gpcm

import (
	"owfc/common"
	"owfc/logging"
	"regexp"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

var isDWCMatchCommand = regexp.MustCompile(`^GPCM\d+vMAT`).MatchString

func (g *GameSpySession) buddyMessage(command common.GameSpyCommand) {
	// TODO: There are other command values that mean the same thing
	if command.CommandValue != strconv.Itoa(BuddyMessage) {
		logging.Error(g.ModuleName, "Received unknown buddy message type:", aurora.Cyan(command.CommandValue))
		return
	}

	strToProfileId := command.OtherValues["t"]
	toProfileId, err := strconv.ParseUint(strToProfileId, 10, 32)
	if err != nil {
		logging.Error(g.ModuleName, "Invalid profile ID string:", aurora.Cyan(strToProfileId))
		g.replyError(ErrMessage)
		return
	}

	if !g.isFriendAuthorized(uint32(toProfileId)) {
		logging.Error(g.ModuleName, "Destination", aurora.Cyan(toProfileId), "is not even on sender's friend list")
		g.replyError(ErrMessageNotFriends)
		return
	}

	msg, ok := command.OtherValues["msg"]
	if !ok || msg == "" {
		logging.Error(g.ModuleName, "Missing message value")
		g.replyError(ErrMessage)
		return
	}

	// DWCi_GetGPBuddyAdditionalMsg copies everything between / into a 16 byte buffer
	// regardless of actual size
	if isDWCMatchCommand(msg) {
		for i, segment := range strings.Split(msg, "/") {
			// first segment is header and message type
			if i == 0 {
				continue
			}

			// segments are uint32 strings, skip if in bounds (10 characters)
			if len(segment) <= 10 {
				continue
			}

			logging.Error(g.ModuleName, "Invalid DWC match command parameter")
			g.replyError(ErrMessage)
			return
		}
	}

	mutex.Lock()
	defer mutex.Unlock()

	var toSession *GameSpySession
	if toSession, ok = sessions[uint32(toProfileId)]; !ok || !toSession.LoggedIn {
		logging.Error(g.ModuleName, "Destination", aurora.Cyan(toProfileId), "is not online")
		g.replyError(ErrMessageFriendOffline)
		return
	}

	if toSession.GameName != g.GameName {
		logging.Error(g.ModuleName, "Destination", aurora.Cyan(toProfileId), "is not playing the same game")
		g.replyError(ErrMessage)
		return
	}

	logging.Notice(g.ModuleName, "Sending buddy message to", aurora.Cyan(toSession.Profile.ID))

	sendMessageToSession(BuddyMessage, g.Profile.ID, toSession, msg)
}

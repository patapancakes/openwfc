package gpcm

import (
	"owfc/common"
	"owfc/logging"
	"strconv"

	"github.com/logrusorgru/aurora/v3"
)

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

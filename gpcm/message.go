package gpcm

import (
	"owfc/common/gamespy"
	"owfc/logging"
	"regexp"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

var isDWCMatchCommand = regexp.MustCompile(`^GPCM\d+vMAT`).MatchString

type BuddyMessageRequest struct {
	Command int    `gs:"bm"`
	SessKey int32  `gs:"sesskey"`
	Target  uint32 `gs:"t"`
	Message string `gs:"msg"`
}

func buddyMessage(state *GameSpySession, req BuddyMessageRequest) (gamespy.NoResponse, error) {
	if !state.LoggedIn {
		return gamespy.NoResponse{}, ErrNotLoggedIn
	}

	// TODO: There are other command values that mean the same thing
	if req.Command != BuddyMessage {
		logging.Error(state.ModuleName, "Received unknown buddy message type:", aurora.Cyan(req.Command))
		return gamespy.NoResponse{}, nil
	}

	if !state.isFriendAuthorized(req.Target) {
		logging.Error(state.ModuleName, "Destination", aurora.Cyan(req.Target), "is not even on sender's friend list")
		return gamespy.NoResponse{}, ErrMessageNotFriends
	}

	// DWCi_GetGPBuddyAdditionalMsg copies everything between / into a 16 byte buffer
	// regardless of actual size
	if isDWCMatchCommand(req.Message) {
		for i, segment := range strings.Split(req.Message, "/") {
			// first segment is header and message type
			if i == 0 {
				continue
			}

			// segments are uint32 strings, skip if in bounds (10 characters)
			if len(segment) <= 10 {
				continue
			}

			logging.Error(state.ModuleName, "Invalid DWC match command parameter")
			return gamespy.NoResponse{}, ErrMessage
		}
	}

	mutex.Lock()
	defer mutex.Unlock()

	toSession, ok := sessions[req.Target]
	if !ok || !toSession.LoggedIn {
		logging.Error(state.ModuleName, "Destination", aurora.Cyan(req.Target), "is not online")
		return gamespy.NoResponse{}, ErrMessageFriendOffline
	}

	if toSession.GameName != state.GameName {
		logging.Error(state.ModuleName, "Destination", aurora.Cyan(req.Target), "is not playing the same game")
		return gamespy.NoResponse{}, ErrMessage
	}

	logging.Notice(state.ModuleName, "Sending buddy message to", aurora.Cyan(toSession.Profile.ID))

	toSession.sendMessage(BuddyMessage, state.Profile.ID, req.Message, false)
	return gamespy.NoResponse{}, nil
}

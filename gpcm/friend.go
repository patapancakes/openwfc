package gpcm

import (
	"database/sql"
	"strconv"
	"strings"
	"wwfc/common"
	"wwfc/logging"
	"wwfc/qr2"

	mysqlerrnum "github.com/bombsimon/mysql-error-numbers/v3"
	"github.com/logrusorgru/aurora/v3"
)

const (
	BuddyMessage = iota + 1
	BuddyRequest
	BuddyReply
	BuddyAuth
	BuddyUTM
	BuddyRevoke
)

const (
	BuddyStatus = iota + 100
	BuddyInvite
	BuddyPing
	BuddyPong
)

func (g *GameSpySession) isFriendAuthorized(profileId uint32) bool {
	friends, err := db.GetFriends(profileId)
	if err != nil && err != sql.ErrNoRows {
		return false
	}

	var authorized bool
	for _, friend := range friends {
		if friend.ID != g.Profile.ID {
			continue
		}

		if !friend.Authorized {
			continue
		}

		authorized = true
		break
	}

	return authorized
}

func (g *GameSpySession) isFriendIncoming(profileId uint32) bool {
	friends, err := db.GetFriends(g.Profile.ID)
	if err != nil && err != sql.ErrNoRows {
		return false
	}

	var incoming bool
	for _, friend := range friends {
		if friend.ID != profileId {
			continue
		}

		incoming = true
		break
	}

	return incoming
}

const (
	addFriendMessage = "\r\n\r\n|signed|00000000000000000000000000000000"

	// Message used by DS games and some Wii games
	bm1AuthMessage = "I have authorized your request to add me to your list"

	offlineMessage = "|s|0|ss|Offline|ls||ip|0|p|0|qm|0"
)

func (g *GameSpySession) isBm1AuthMessageNeeded() bool {
	return g.UnitCode == UnitCodeDS || g.UnitCode == UnitCodeDSAndWii || g.GameName == "jissenpachwii" || g.GameName == "drmariowii" || g.GameName == "pokebattlewii"
}

func (g *GameSpySession) addFriend(command common.GameSpyCommand) {
	newProfileId, err := strconv.Atoi(command.OtherValues["newprofileid"])
	if err != nil {
		g.replyError(ErrAddFriend)
		return
	}

	if newProfileId == int(g.Profile.ID) {
		logging.Error(g.ModuleName, "Attempt to add self as friend")
		g.replyError(ErrAddFriendBadNew)
		return
	}

	fc := common.CalcFriendCodeString(uint32(newProfileId), g.Profile.GsbrCode[:4])
	logging.Info(g.ModuleName, "Add friend:", aurora.Cyan(newProfileId), aurora.Cyan(fc))

	err = db.AddFriend(g.Profile.ID, uint32(newProfileId))
	if err != nil {
		switch mysqlerrnum.FromError(err) {
		case mysqlerrnum.ErrDupEntry:
			if g.isFriendAuthorized(uint32(newProfileId)) {
				logging.Info(g.ModuleName, "Attempt to add a friend twice")
				g.replyError(ErrAddFriendAlreadyFriends)
			}

			return
		case mysqlerrnum.ErrNoReferencedRow2:
			logging.Info(g.ModuleName, "Attempt to add a non-existent friend")
			g.replyError(ErrAddFriendBadNew)
		default:
			logging.Info(g.ModuleName, err)
			g.replyError(ErrAddFriend)
		}

		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	recipient, ok := sessions[uint32(newProfileId)]
	recipientOnline := ok && recipient != nil && recipient.LoggedIn

	if recipientOnline && recipient.GameName != g.GameName {
		// TODO: check this for offline recipients too
		logging.Error(g.ModuleName, "Destination is not playing the same game")
		g.replyError(ErrAddFriendBadNew)
		return
	}

	// if both are friends now
	if g.isFriendAuthorized(uint32(newProfileId)) {
		if recipientOnline {
			// notify recipient
			g.exchangeFriendStatus(recipient.Profile.ID)

			if recipient.isBm1AuthMessageNeeded() {
				sendMessageToSession(BuddyMessage, g.Profile.ID, recipient, bm1AuthMessage)
			}

			sendMessageToSession(BuddyAuth, g.Profile.ID, recipient, "")
		}

		// notify sender
		if !recipientOnline {
			sendMessageToSession(BuddyStatus, uint32(newProfileId), g, offlineMessage)
		}

		if g.isBm1AuthMessageNeeded() {
			sendMessageToSession(BuddyMessage, uint32(newProfileId), g, bm1AuthMessage)
		}

		sendMessageToSession(BuddyAuth, uint32(newProfileId), g, "")
		return
	}

	if !recipientOnline {
		logging.Info(g.ModuleName, "Destination is not online")
		return
	}

	// notify recipient of the friend request
	sendMessageToSession(BuddyRequest, g.Profile.ID, recipient, addFriendMessage)
}

func (g *GameSpySession) removeFriend(command common.GameSpyCommand) {
	delProfileID, err := strconv.Atoi(command.OtherValues["delprofileid"])
	if err != nil {
		logging.Error(g.ModuleName, aurora.Cyan(delProfileID), "is not a valid profile id")
		g.replyError(ErrDeleteFriend)
		return
	}

	delProfileID32 := uint32(delProfileID)

	fc := common.CalcFriendCodeString(delProfileID32, g.Profile.GsbrCode[:4])
	logging.Info(g.ModuleName, "Remove friend:", aurora.Cyan(delProfileID), aurora.Cyan(fc))

	// get authorized status before we remove
	authorized := g.isFriendAuthorized(delProfileID32)

	revoked, err := db.RemoveFriend(g.Profile.ID, delProfileID32)
	if err != nil {
		g.replyError(ErrDeleteFriend)
		return
	}
	if !revoked {
		g.replyError(ErrRevokeNotFriends)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	if recipient, ok := sessions[delProfileID32]; ok && recipient.LoggedIn && authorized {
		sendMessageToSession(BuddyRevoke, g.Profile.ID, recipient, "")
	}
}

func (g *GameSpySession) authAddFriend(command common.GameSpyCommand) {
	fromProfileId, err := strconv.Atoi(command.OtherValues["fromprofileid"])
	if err != nil {
		logging.Error(g.ModuleName, "Invalid profile ID string:", aurora.Cyan(fromProfileId))
		g.replyError(ErrAuthAddBadFrom)
		return
	}

	if !g.isFriendIncoming(uint32(fromProfileId)) {
		logging.Error(g.ModuleName, "Sender", aurora.Cyan(fromProfileId), "is not an incoming friend")
		g.replyError(ErrAuthAddBadFrom)
		return
	}

	err = db.AddFriend(g.Profile.ID, uint32(fromProfileId))
	if err != nil {
		if mysqlerrnum.FromError(err) == mysqlerrnum.ErrDupEntry {
			return
		}

		g.replyError(ErrAuthAdd)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	recipient, ok := sessions[uint32(fromProfileId)]
	if !ok || recipient == nil || !recipient.LoggedIn {
		logging.Info(g.ModuleName, "Destination is not online")
		return
	}

	sendMessageToSession(BuddyRequest, recipient.Profile.ID, g, addFriendMessage)
}

func (g *GameSpySession) setStatus(command common.GameSpyCommand) {
	status, err := strconv.Atoi(command.CommandValue)
	if err != nil {
		logging.Error(g.ModuleName, "Invalid status value")
		g.replyError(ErrStatus)
		return
	}

	logging.Notice(g.ModuleName, "New status:", aurora.BrightMagenta(common.GetStatusString(status)))

	qr2.ProcessGPStatusUpdate(g.Profile.ID, g.QR2IP, status)

	statstring, ok := command.OtherValues["statstring"]
	if !ok {
		logging.Warn(g.ModuleName, "Missing statstring")
		statstring = ""
	}

	locstring, ok := command.OtherValues["locstring"]
	if !ok {
		logging.Warn(g.ModuleName, "Missing locstring")
		locstring = ""
	}

	statusMsg := "|s|" + strconv.Itoa(status) + "|ss|" + statstring + "|ls|" + locstring + "|ip|0|p|0|qm|0"

	mutex.Lock()
	defer mutex.Unlock()

	if status == common.MatchAnybody && g.Profile.Restricted {
		logging.Warn(g.ModuleName, "Restricted profile searching for public rooms")
		kickPlayer(g.Profile.ID, "restricted_join")
	}

	g.LocString = locstring
	g.Status = statusMsg

	friends, err := db.GetFriends(g.Profile.ID)
	if err != nil {
		return
	}
	for _, friend := range friends {
		if !friend.Authorized {
			continue
		}

		g.sendFriendStatus(friend.ID, false)
	}
}

func sendMessageToSession(msgType int, from uint32, session *GameSpySession, msg string) {
	message := common.CreateGameSpyMessage(common.GameSpyCommand{
		Command:      "bm",
		CommandValue: strconv.Itoa(msgType),
		OtherValues: map[string]string{
			"f":   strconv.FormatUint(uint64(from), 10),
			"msg": msg,
		},
	})
	if err := common.SendPacket(ServerName, session.ConnIndex, []byte(message)); err != nil {
		logging.Error("GPCM", "Failed to send packet:", err)
		_ = common.CloseConnection(ServerName, session.ConnIndex)
	}
}

func sendMessageToSessionBuffer(msgType int, from uint32, session *GameSpySession, msg string) {
	session.WriteBuffer += common.CreateGameSpyMessage(common.GameSpyCommand{
		Command:      "bm",
		CommandValue: strconv.Itoa(msgType),
		OtherValues: map[string]string{
			"f":   strconv.FormatUint(uint64(from), 10),
			"msg": msg,
		},
	})
}

func (g *GameSpySession) sendFriendStatus(profileId uint32, buffer bool) {
	recipient, ok := sessions[profileId]
	if !ok || !recipient.LoggedIn {
		return
	}

	// Prevent players abusing a stack overflow exploit with the locstring in Mario Kart Wii
	if strings.HasPrefix(recipient.GameCode, "RMC") && len(g.LocString) > 0x14 {
		logging.Warn("GPCM", "Blocked message from", aurora.Cyan(g.Profile.ID), "to", aurora.Cyan(recipient.Profile.ID), "due to a stack overflow exploit")
		return
	}

	f := sendMessageToSession
	if buffer {
		f = sendMessageToSessionBuffer
	}

	f(BuddyStatus, g.Profile.ID, recipient, g.Status)
}

func (g *GameSpySession) exchangeFriendStatus(profileId uint32) {
	recipient, ok := sessions[profileId]
	if !ok || !recipient.LoggedIn {
		return
	}

	// to recipient
	if strings.HasPrefix(recipient.GameCode, "RMC") && len(g.LocString) > 0x14 {
		logging.Warn("GPCM", "Blocked message from", aurora.Cyan(g.Profile.ID), "to", aurora.Cyan(recipient.Profile.ID), "due to a stack overflow exploit")
		return
	}

	sendMessageToSession(BuddyStatus, g.Profile.ID, recipient, g.Status)

	// to sender
	if strings.HasPrefix(g.GameCode, "RMC") && len(recipient.LocString) > 0x14 {
		logging.Warn("GPCM", "Blocked message from", aurora.Cyan(recipient.Profile.ID), "to", aurora.Cyan(g.Profile.ID), "due to a stack overflow exploit")
		return
	}

	sendMessageToSession(BuddyStatus, profileId, g, recipient.Status)
}

func (g *GameSpySession) sendLogoutStatus() {
	mutex.Lock()
	defer mutex.Unlock()

	friends, err := db.GetFriends(g.Profile.ID)
	if err != nil {
		return
	}
	for _, friend := range friends {
		if !friend.Authorized {
			continue
		}

		recipient, ok := sessions[friend.ID]
		if !ok || !recipient.LoggedIn {
			return
		}

		sendMessageToSession(BuddyStatus, g.Profile.ID, recipient, offlineMessage)
	}
}

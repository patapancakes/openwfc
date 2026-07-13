package gpcm

import (
	"slices"
	"strconv"
	"strings"
	"wwfc/common"
	"wwfc/logging"
	"wwfc/qr2"

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

func removeFromUint32Array(arrayPointer *[]uint32, index int) {
	array := *arrayPointer
	arrayLength := len(array)

	if index < 0 || index >= arrayLength {
		return // Ignore?
	}

	lastIndex := arrayLength - 1

	array[index] = array[lastIndex]
	*arrayPointer = array[:lastIndex]
}

func (g *GameSpySession) isFriendAdded(profileId uint32) bool {
	return slices.Contains(g.FriendList, profileId)
}

func (g *GameSpySession) getFriendIndex(profileId uint32) int {
	for i, storedPid := range g.FriendList {
		if storedPid == profileId {
			return i
		}
	}
	return -1
}

func (g *GameSpySession) isFriendAuthorized(profileId uint32) bool {
	return slices.Contains(g.AuthFriendList, profileId)
}

func (g *GameSpySession) getAuthorizedFriendIndex(profileId uint32) int {
	for i, storedPid := range g.AuthFriendList {
		if storedPid == profileId {
			return i
		}
	}
	return -1
}

const (
	addFriendMessage = "\r\n\r\n|signed|00000000000000000000000000000000"

	// Message used by DS games and some Wii games
	bm1AuthMessage = "I have authorized your request to add me to your list"

	logOutMessage = "|s|0|ss|Offline|ls||ip|0|p|0|qm|0"
)

func (g *GameSpySession) isBm1AuthMessageNeeded() bool {
	return g.UnitCode == UnitCodeDS || g.UnitCode == UnitCodeDSAndWii || g.GameName == "jissenpachwii" || g.GameName == "drmariowii" || g.GameName == "pokebattlewii"
}

func (g *GameSpySession) addFriend(command common.GameSpyCommand) {
	strNewProfileId := command.OtherValues["newprofileid"]
	newProfileId, err := strconv.ParseUint(strNewProfileId, 10, 32)
	if err != nil {
		g.replyError(ErrAddFriend)
		return
	}

	if newProfileId == uint64(g.Profile.ID) {
		logging.Error(g.ModuleName, "Attempt to add self as friend")
		g.replyError(ErrAddFriendBadNew)
		return
	}

	fc := common.CalcFriendCodeString(uint32(newProfileId), g.Profile.GsbrCode[:4])
	logging.Info(g.ModuleName, "Add friend:", aurora.Cyan(strNewProfileId), aurora.Cyan(fc))

	mutex.Lock()
	defer mutex.Unlock()

	authorized := g.isFriendAuthorized(uint32(newProfileId))
	if authorized {
		logging.Info(g.ModuleName, "Attempt to add a friend who is already authorized")
		// This seems to always happen, do we need to return an error?
		// DWC vocally ignores the error anyway, so let's not bother
		// g.replyError(ErrAddFriendAlreadyFriends)
		return
	}

	// TODO: Add a limit
	if !g.isFriendAdded(uint32(newProfileId)) {
		g.FriendList = append(g.FriendList, uint32(newProfileId))
	}

	// Check if destination has added the sender
	newSession, ok := sessions[uint32(newProfileId)]
	if !ok || newSession == nil || !newSession.LoggedIn {
		logging.Info(g.ModuleName, "Destination is not online")
		return
	}

	if newSession.GameName != g.GameName {
		logging.Error(g.ModuleName, "Destination is not playing the same game")
		// g.replyError(ErrAddFriendBadNew)
		return
	}

	if !newSession.isFriendAdded(g.Profile.ID) {
		// Not an error, just ignore for now
		logging.Info(g.ModuleName, "Destination has not added sender")
		sendMessageToSessionBuffer(BuddyRequest, g.Profile.ID, newSession, addFriendMessage)
		return
	}

	// Friends are now mutual!
	// TODO: Add a limit
	if !authorized {
		g.AuthFriendList = append(g.AuthFriendList, uint32(newProfileId))
		newSession.AuthFriendList = append(newSession.AuthFriendList, g.Profile.ID)
	}

	// Send friend auth message
	sendMessageToSessionBuffer(BuddyAuth, newSession.Profile.ID, g, "")

	if g.isBm1AuthMessageNeeded() {
		sendMessageToSessionBuffer(BuddyMessage, newSession.Profile.ID, g, bm1AuthMessage)
	}

	if newSession.isFriendAdded(g.Profile.ID) {
		sendMessageToSession(BuddyAuth, g.Profile.ID, newSession, "")

		if newSession.isBm1AuthMessageNeeded() {
			sendMessageToSession(BuddyMessage, g.Profile.ID, newSession, bm1AuthMessage)
		}

		g.sendFriendStatus(newSession.Profile.ID)
	}

	newSession.sendFriendStatus(g.Profile.ID)
}

func (g *GameSpySession) removeFriend(command common.GameSpyCommand) {
	strDelProfileID := command.OtherValues["delprofileid"]
	delProfileID64, err := strconv.ParseUint(strDelProfileID, 10, 32)
	if err != nil {
		logging.Error(g.ModuleName, aurora.Cyan(strDelProfileID), "is not a valid profile id")
		g.replyError(ErrDeleteFriend)
		return
	}
	delProfileID32 := uint32(delProfileID64)

	fc := common.CalcFriendCodeString(delProfileID32, g.Profile.GsbrCode[:4])
	logging.Info(g.ModuleName, "Remove friend:", aurora.Cyan(strDelProfileID), aurora.Cyan(fc))

	mutex.Lock()
	defer mutex.Unlock()

	if g.isFriendAdded(delProfileID32) {
		delProfileIDIndex := g.getFriendIndex(delProfileID32)
		removeFromUint32Array(&g.FriendList, delProfileIDIndex)
	}

	if g.isFriendAuthorized(delProfileID32) {
		delProfileIDIndex := g.getAuthorizedFriendIndex(delProfileID32)
		removeFromUint32Array(&g.AuthFriendList, delProfileIDIndex)
	}

	if session, ok := sessions[delProfileID32]; ok && session.LoggedIn && session.isFriendAuthorized(g.Profile.ID) {
		sendMessageToSessionBuffer(BuddyStatus, g.Profile.ID, session, logOutMessage) // TODO: find out of this is needed
		sendMessageToSessionBuffer(BuddyRevoke, g.Profile.ID, session, "")
	}
}

func (g *GameSpySession) authAddFriend(command common.GameSpyCommand) {
	strFromProfileId := command.OtherValues["fromprofileid"]
	fromProfileId, err := strconv.ParseUint(strFromProfileId, 10, 32)
	if err != nil {
		logging.Error(g.ModuleName, "Invalid profile ID string:", aurora.Cyan(strFromProfileId))
		g.replyError(ErrAuthAddBadFrom)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	if !g.isFriendAuthorized(uint32(fromProfileId)) {
		logging.Error(g.ModuleName, "Sender", aurora.Cyan(fromProfileId), "is not an authorized friend")
		g.replyError(ErrAuthAddBadFrom)
		return
	}

	g.exchangeFriendStatus(uint32(fromProfileId))
}

func (g *GameSpySession) setStatus(command common.GameSpyCommand) {
	status, err := strconv.Atoi(command.CommandValue)
	if err != nil {
		logging.Error(g.ModuleName, "Invalid status value")
		g.replyError(ErrStatus)
		return
	}

	logging.Notice(g.ModuleName, "New status:", aurora.BrightMagenta(status))

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

	for _, storedPid := range g.AuthFriendList {
		g.sendFriendStatus(storedPid)
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

func sendMessageToProfileId(msgType int, from uint32, to uint32, msg string) bool {
	if session, ok := sessions[to]; ok && session.LoggedIn {
		sendMessageToSession(msgType, from, session, msg)
		return true
	}

	logging.Info("GPCM", "Destination", aurora.Cyan(to), "from", aurora.Cyan(from), "is not online")
	return false
}

func (g *GameSpySession) sendFriendStatus(profileId uint32) {
	common.MaybeUnused(sendMessageToProfileId)

	if !g.isFriendAuthorized(profileId) {
		return
	}

	if session, ok := sessions[profileId]; ok && session.LoggedIn && session.isFriendAdded(g.Profile.ID) {
		// Prevent players abusing a stack overflow exploit with the locstring in Mario Kart Wii
		if strings.HasPrefix(session.GameCode, "RMC") && len(g.LocString) > 0x14 {
			logging.Warn("GPCM", "Blocked message from", aurora.Cyan(g.Profile.ID), "to", aurora.Cyan(session.Profile.ID), "due to a stack overflow exploit")
			return
		}

		session.recordStatusSent(g.Profile.ID)
		sendMessageToSession(BuddyStatus, g.Profile.ID, session, g.Status)
	}
}

func (g *GameSpySession) exchangeFriendStatus(profileId uint32) {
	if session, ok := sessions[profileId]; ok && session.LoggedIn {
		if session.isFriendAdded(g.Profile.ID) && session.isFriendAuthorized(g.Profile.ID) {
			if strings.HasPrefix(session.GameCode, "RMC") && len(g.LocString) > 0x14 {
				logging.Warn("GPCM", "Blocked message from", aurora.Cyan(g.Profile.ID), "to", aurora.Cyan(session.Profile.ID), "due to a stack overflow exploit")
				return
			}

			session.recordStatusSent(g.Profile.ID)
			sendMessageToSession(BuddyStatus, g.Profile.ID, session, g.Status)
		}

		if g.isFriendAdded(profileId) && g.isFriendAuthorized(profileId) {
			if strings.HasPrefix(g.GameCode, "RMC") && len(session.LocString) > 0x14 {
				logging.Warn("GPCM", "Blocked message from", aurora.Cyan(session.Profile.ID), "to", aurora.Cyan(g.Profile.ID), "due to a stack overflow exploit")
				return
			}

			g.recordStatusSent(profileId)
			sendMessageToSessionBuffer(BuddyStatus, profileId, g, session.Status)
		}
	}
}

func (g *GameSpySession) recordStatusSent(sender uint32) {
	if slices.Contains(g.RecvStatusFromList, sender) {
		return
	}

	g.RecvStatusFromList = append(g.RecvStatusFromList, sender)
}

func (g *GameSpySession) sendLogoutStatus() {
	mutex.Lock()
	defer mutex.Unlock()

	for _, storedPid := range g.AuthFriendList {
		if session, ok := sessions[storedPid]; ok && session.LoggedIn && session.isFriendAuthorized(g.Profile.ID) {
			delProfileIDIndex := session.getAuthorizedFriendIndex(g.Profile.ID)
			removeFromUint32Array(&session.AuthFriendList, delProfileIDIndex)
			sendMessageToSession(BuddyStatus, g.Profile.ID, session, logOutMessage)
		}
	}
}

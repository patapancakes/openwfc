package gpcm

import (
	"owfc/common"
	"owfc/common/gamespy"
	"owfc/logging"
	"strconv"
	"strings"

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
	authorized, err := db.GetFriendAuth(g.Profile.ID, profileId)
	if err != nil {
		return false
	}

	return authorized
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

type AddBuddyRequest struct {
	Command      string `gs:"addbuddy"`
	SessKey      int32  `gs:"sesskey"`
	NewProfileID uint32 `gs:"newprofileid"`
	Reason       string `gs:"reason"`
}

func addBuddy(state *GameSpySession, req AddBuddyRequest) (gamespy.NoResponse, error) {
	if !state.LoggedIn {
		return gamespy.NoResponse{}, ErrNotLoggedIn
	}

	if req.NewProfileID == state.Profile.ID {
		logging.Error(state.ModuleName, "Attempt to add self as friend")
		return gamespy.NoResponse{}, ErrAddFriendBadNew
	}

	fc := common.CalcFriendCodeString(req.NewProfileID, state.Profile.GsbrCode[:4])
	logging.Info(state.ModuleName, "Add friend:", aurora.Cyan(req.NewProfileID), aurora.Cyan(fc))

	err := db.AddFriend(state.Profile.ID, req.NewProfileID)
	if err != nil {
		switch mysqlerrnum.FromError(err) {
		case mysqlerrnum.ErrDupEntry:
			logging.Info(state.ModuleName, "Attempt to add a friend twice")
			return gamespy.NoResponse{}, ErrAddFriendAlreadyFriends
		case mysqlerrnum.ErrNoReferencedRow2:
			logging.Info(state.ModuleName, "Attempt to add a non-existent friend")
			return gamespy.NoResponse{}, ErrAddFriendBadNew
		}

		logging.Info(state.ModuleName, err)
		return gamespy.NoResponse{}, ErrAddFriend
	}

	mutex.Lock()
	defer mutex.Unlock()

	recipient, ok := sessions[req.NewProfileID]
	if !ok || recipient == nil || !recipient.LoggedIn {
		logging.Info(state.ModuleName, "Destination is not online")
		return gamespy.NoResponse{}, nil
	}

	// notify recipient of the friend request
	recipient.sendMessage(BuddyRequest, state.Profile.ID, addFriendMessage, false)
	return gamespy.NoResponse{}, nil
}

type DelBuddyRequest struct {
	Command      string `gs:"delbuddy"`
	SessKey      int32  `gs:"sesskey"`
	DelProfileID uint32 `gs:"delprofileid"`
}

func delBuddy(state *GameSpySession, req DelBuddyRequest) (gamespy.NoResponse, error) {
	if !state.LoggedIn {
		return gamespy.NoResponse{}, ErrNotLoggedIn
	}

	fc := common.CalcFriendCodeString(req.DelProfileID, state.Profile.GsbrCode[:4])
	logging.Info(state.ModuleName, "Remove friend:", aurora.Cyan(req.DelProfileID), aurora.Cyan(fc))

	// get authorized status before we remove
	authorized := state.isFriendAuthorized(req.DelProfileID)

	revoked, err := db.RemoveFriend(state.Profile.ID, req.DelProfileID)
	if err != nil {
		return gamespy.NoResponse{}, ErrDeleteFriend
	}
	if !revoked {
		return gamespy.NoResponse{}, ErrRevokeNotFriends
	}

	mutex.Lock()
	defer mutex.Unlock()

	recipient, ok := sessions[req.DelProfileID]
	if !ok || !recipient.LoggedIn || !authorized {
		return gamespy.NoResponse{}, nil
	}

	recipient.sendMessage(BuddyRevoke, state.Profile.ID, "", false)
	return gamespy.NoResponse{}, nil
}

type AuthAddRequest struct {
	Command       string `gs:"authadd"`
	SessKey       int32  `gs:"sesskey"`
	FromProfileID uint32 `gs:"fromprofileid"`
	Signature     string `gs:"sig"`
	AutoSync      bool   `gs:"autoSync"`
}

func authAdd(state *GameSpySession, req AuthAddRequest) (gamespy.NoResponse, error) {
	if !state.LoggedIn {
		return gamespy.NoResponse{}, ErrNotLoggedIn
	}

	err := db.AuthFriend(req.FromProfileID, state.Profile.ID)
	if err != nil {
		logging.Error(state.ModuleName, "Sender", aurora.Cyan(req.FromProfileID), "is not an incoming friend")
		return gamespy.NoResponse{}, ErrAuthAddBadFrom
	}

	mutex.Lock()
	defer mutex.Unlock()

	recipient, ok := sessions[req.FromProfileID]
	if !ok || recipient == nil || !recipient.LoggedIn {
		logging.Info(state.ModuleName, "Destination is not online")
		return gamespy.NoResponse{}, nil
	}

	// TODO: see if this should go last
	state.sendFriendStatus(recipient.Profile.ID, false)

	recipient.sendMessage(BuddyAuth, state.Profile.ID, "", false)

	if recipient.isBm1AuthMessageNeeded() {
		recipient.sendMessage(BuddyMessage, state.Profile.ID, bm1AuthMessage, false)
	}

	return gamespy.NoResponse{}, nil
}

type StatusRequest struct {
	Command  int    `gs:"status"`
	SessKey  int32  `gs:"sesskey"`
	Status   string `gs:"statstring"`
	Location string `gs:"locstring"`
}

func status(state *GameSpySession, req StatusRequest) (gamespy.NoResponse, error) {
	if !state.LoggedIn {
		return gamespy.NoResponse{}, ErrNotLoggedIn
	}

	if len(req.Status) >= 256 {
		logging.Warn(state.ModuleName, "Invalid statstring")
		return gamespy.NoResponse{}, ErrStatus
	}

	if len(req.Location) >= 256 {
		logging.Warn(state.ModuleName, "Invalid locstring")
		return gamespy.NoResponse{}, ErrStatus
	}

	state.LocString = req.Location

	ip, _ := common.IPFormatToString(state.RemoteAddr)
	state.Status = "|s|" + strconv.Itoa(req.Command) + "|ss|" + req.Status + "|ls|" + req.Location + "|ip|" + ip + "|p|0|qm|0"

	mutex.Lock()
	defer mutex.Unlock()

	friends, err := db.GetFriends(state.Profile.ID, false)
	if err != nil {
		return gamespy.NoResponse{}, err
	}
	for _, friend := range friends {
		if !friend.Authorized {
			continue
		}

		state.sendFriendStatus(friend.ID, false)
	}

	return gamespy.NoResponse{}, nil
}

func (g *GameSpySession) sendMessage(msgType int, from uint32, msg string, buffer bool) {
	type BuddyMessage struct {
		Command int    `gs:"bm"`
		Sender  uint32 `gs:"f"`
		Message string `gs:"msg"`
	}

	message := gamespy.Marshal(BuddyMessage{
		Command: msgType,
		Sender:  from,
		Message: msg,
	})

	if buffer {
		g.WriteBuffer += message
		return
	}

	err := common.SendPacket(ServerName, g.ConnIndex, []byte(message))
	if err != nil {
		logging.Error("GPCM", "Failed to send packet:", err)
		_ = common.CloseConnection(ServerName, g.ConnIndex)
	}
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

	recipient.sendMessage(BuddyStatus, g.Profile.ID, g.Status, buffer)
}

func (g *GameSpySession) sendLogoutStatus() {
	mutex.Lock()
	defer mutex.Unlock()

	friends, err := db.GetFriends(g.Profile.ID, false)
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

		recipient.sendMessage(BuddyStatus, g.Profile.ID, offlineMessage, false)
	}
}

func (g *GameSpySession) sendFriendsInfo() {
	// send status for unauthorized outgoing friend requests
	outgoing, err := db.GetFriends(g.Profile.ID, true)
	if err == nil {
		for _, friend := range outgoing {
			if friend.Authorized {
				continue
			}

			// TODO: see if it should send their online status
			g.sendMessage(BuddyStatus, friend.ID, offlineMessage, true)
		}
	}

	// send status for incoming friend requests / mutual friends
	friends, err := db.GetFriends(g.Profile.ID, false)
	if err != nil {
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	for _, friend := range friends {
		if !friend.Authorized {
			g.sendMessage(BuddyRequest, friend.ID, addFriendMessage, true)
			continue
		}

		session, ok := sessions[friend.ID]
		if !ok || !session.LoggedIn {
			g.sendMessage(BuddyStatus, friend.ID, offlineMessage, true)
			continue
		}

		session.sendFriendStatus(g.Profile.ID, false)
	}
}

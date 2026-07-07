package gpcm

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"wwfc/common"
	"wwfc/database"
	"wwfc/logging"
	"wwfc/qr2"

	"github.com/logrusorgru/aurora/v3"
)

const (
	UnitCodeDS       = 0
	UnitCodeWii      = 1
	UnitCodeDSAndWii = 0xff
)

func generateResponse(gpcmChallenge, nasChallenge, authToken, clientChallenge string) string {
	hasher := md5.New()
	hasher.Write([]byte(nasChallenge))
	str := hex.EncodeToString(hasher.Sum(nil))
	str += strings.Repeat(" ", 48)
	str += authToken
	str += clientChallenge
	str += gpcmChallenge
	str += hex.EncodeToString(hasher.Sum(nil))

	_hasher := md5.New()
	_hasher.Write([]byte(str))
	return hex.EncodeToString(_hasher.Sum(nil))
}

func generateProof(gpcmChallenge, nasChallenge, authToken, clientChallenge string) string {
	return generateResponse(clientChallenge, nasChallenge, authToken, gpcmChallenge)
}

func (g *GameSpySession) login(command common.GameSpyCommand) {
	if g.LoggedIn {
		logging.Error(g.ModuleName, "Attempt to login twice")
		g.replyError(ErrLogin)
		return
	}

	authToken := command.OtherValues["authtoken"]
	if authToken == "" {
		g.replyError(ErrLogin)
		return
	}

	authTokenObj := common.NASAuthToken{}
	err := authTokenObj.Unmarshal(authToken)
	if err != nil {
		logging.Error(g.ModuleName, "Failed to unmarshal auth token:", err)
		if err == common.ErrTokenExpired {
			g.replyError(ErrLoginLoginTicketExpired)
			return
		}
		g.replyError(ErrLogin)
		return
	}

	g.GameName = command.OtherValues["gamename"]
	logging.Info(g.ModuleName, "Game name:", aurora.Cyan(g.GameName))
	g.GameCode = common.NullTerminatedString(authTokenObj.GameCode[:])
	g.Region = authTokenObj.Region
	g.Language = authTokenObj.Lang
	g.ConsoleFriendCode = authTokenObj.ConsoleFriendCode
	g.UnitCode = authTokenObj.UnitCode

	var endianness binary.ByteOrder = binary.LittleEndian
	if g.UnitCode == UnitCodeWii {
		endianness = binary.BigEndian
	}

	g.InGameName = common.UTF16Decode(authTokenObj.InGameScreenName[:], endianness)

	if g.UnitCode == UnitCodeDS {
		g.HostPlatform = "DS"
	} else {
		g.HostPlatform = "Wii"
	}

	g.LoginInfoSet = true

	logging.Event(
		"received_login_info",
		map[string]any{
			"user_id":      authTokenObj.UserID,
			"game_name":    g.GameName,
			"wii_number":   g.ConsoleFriendCode,
			"in_game_name": g.InGameName,
			"unit_code":    g.UnitCode,
			"ip_address":   g.RemoteAddr,
		},
	)

	expectedUnitCode := common.GetExpectedUnitCode(g.GameName)
	if (g.UnitCode != UnitCodeDS && g.UnitCode != UnitCodeWii) || (g.UnitCode != expectedUnitCode && expectedUnitCode != UnitCodeDSAndWii) {
		logging.Error(g.ModuleName, "Incorrect unit code specified:", aurora.Cyan(g.UnitCode))
		g.replyError(ErrLogin)
		return
	}

	nasChallenge := common.NullTerminatedString(authTokenObj.Challenge[:])

	response := generateResponse(g.Challenge, nasChallenge, authToken, command.OtherValues["challenge"])
	if response != command.OtherValues["response"] {
		g.replyError(ErrLogin)
		return
	}

	proof := generateProof(g.Challenge, nasChallenge, command.OtherValues["authtoken"], command.OtherValues["challenge"])

	cmdProfileId := uint32(0)
	if cmdProfileIdStr, exists := command.OtherValues["profileid"]; exists {
		cmdProfileId2, err := strconv.ParseUint(cmdProfileIdStr, 10, 32)
		if err != nil {
			g.replyError(GPError{
				ErrorCode:   ErrLogin.ErrorCode,
				ErrorString: "The provided profile ID is invalid.",
				Fatal:       true,
			})
			return
		}

		cmdProfileId = uint32(cmdProfileId2)
	}

	if !g.performLoginWithDatabase(authTokenObj.UserID, common.NullTerminatedString(authTokenObj.GsbrCode[:]), cmdProfileId) {
		return
	}

	if g.Profile.Created {
		logging.Event(
			"profile_created",
			map[string]any{
				"user_id":    g.Profile.UserID,
				"profile_id": g.Profile.ID,
				"game_name":  g.GameName,
				"ip_address": g.RemoteAddr,
			},
		)

	}

	g.ModuleName = "GPCM:" + strconv.FormatInt(int64(g.Profile.ID), 10) + "*"
	g.ModuleName += "/" + common.CalcFriendCodeString(g.Profile.ID, g.Profile.GsbrCode[:4]) + "*"

	// Check to see if a session is already open with this profile ID
	mutex.Lock()
	otherSession, exists := sessions[g.Profile.ID]
	if exists {
		otherSession.replyError(ErrForcedDisconnect)

		for i := 0; ; i++ {
			mutex.Unlock()
			time.Sleep(300 * time.Millisecond)
			mutex.Lock()

			if _, exists = sessions[g.Profile.ID]; !exists {
				break
			}

			// Give up after 6 seconds
			if i >= 20 {
				mutex.Unlock()
				logging.Error(g.ModuleName, "Failed to disconnect other session")
				g.replyError(ErrForcedDisconnect)
				return
			}
		}
	}
	sessions[g.Profile.ID] = g
	mutex.Unlock()

	g.AuthToken = authToken
	g.LoginTicket = common.GPCMLoginTicket{ProfileID: g.Profile.ID}.Marshal()
	g.SessionKey = rand.Int31n(290000000) + 10000000

	g.DeviceAuthenticated = true
	g.LoggedIn = true

	g.ModuleName = "GPCM:" + strconv.FormatInt(int64(g.Profile.ID), 10)
	g.ModuleName += "/" + common.CalcFriendCodeString(g.Profile.ID, g.Profile.GsbrCode[:4])

	// Notify QR2 of the login
	qr2.Login(g.Profile.ID, g.GameCode, g.InGameName, g.ConsoleFriendCode, g.Profile.GsbrCode[:4], g.RemoteAddr, g.DeviceAuthenticated, g.Profile.Restricted)

	replyUserId := g.Profile.UserID
	if g.UnitCode == UnitCodeDS {
		// Workaround for SDK bug
		replyUserId = 0
	}

	otherValues := map[string]string{
		"sesskey":    strconv.FormatInt(int64(g.SessionKey), 10),
		"proof":      proof,
		"userid":     strconv.FormatUint(replyUserId, 10),
		"profileid":  strconv.FormatUint(uint64(g.Profile.ID), 10),
		"uniquenick": g.Profile.UniqueNick(),
		"lt":         g.LoginTicket,
		"id":         command.OtherValues["id"],
	}

	payload := common.CreateGameSpyMessage(common.GameSpyCommand{
		Command:      "lc",
		CommandValue: "2",
		OtherValues:  otherValues,
	})

	if err := common.SendPacket(ServerName, g.ConnIndex, []byte(payload)); err != nil {
		logging.Error("GPCM", "Failed to send login response packet")
		panic(err)
	}

	logging.Event(
		"logged_in",
		map[string]any{
			"profile_id":   g.Profile.ID,
			"game_name":    g.GameName,
			"in_game_name": g.InGameName,
			"ip_address":   g.RemoteAddr,
		},
	)
}

func (g *GameSpySession) performLoginWithDatabase(userId uint64, gsbrCode string, profileId uint32) bool {
	// Get IP address without port
	ipAddress := g.RemoteAddr
	if strings.Contains(ipAddress, ":") {
		ipAddress = ipAddress[:strings.Index(ipAddress, ":")]
	}

	profile, err := db.LoginUserToGPCM(userId, gsbrCode, profileId, ipAddress, g.InGameName)
	g.Profile = profile

	if err != nil {
		logging.Error(g.ModuleName, "DB error:", err)

		switch err {
		case database.ErrProfileIDInUse:
			g.replyError(GPError{
				ErrorCode:   ErrLogin.ErrorCode,
				ErrorString: "The profile ID is already in use.",
				Fatal:       true,
			})
		case database.ErrReservedProfileIDRange:
			g.replyError(GPError{
				ErrorCode:   ErrLogin.ErrorCode,
				ErrorString: "The profile ID is in a reserved range.",
				Fatal:       true,
			})
		case database.ErrProfileBannedTOS:
			g.replyError(GPError{
				ErrorCode:   ErrLogin.ErrorCode,
				ErrorString: "The profile is banned from the service. Reason: " + profile.BanReason,
				Fatal:       true,
			})
		default:
			g.replyError(GPError{
				ErrorCode:   ErrLogin.ErrorCode,
				ErrorString: "There was an error logging in to the GP backend.",
				Fatal:       true,
			})
		}

		return false
	}

	return true
}

func IsLoggedIn(profileID uint32) bool {
	mutex.Lock()
	defer mutex.Unlock()

	session, exists := sessions[profileID]
	return exists && session.LoggedIn
}

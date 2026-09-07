package gpcm

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	"owfc/common"
	"owfc/logging"
	"strconv"
	"strings"
	"time"

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

type LoginRequest struct {
	Command     string `gs:"login"`
	Challenge   string `gs:"challenge"`
	AuthToken   string `gs:"authtoken"`
	Response    string `gs:"response"`
	Firewall    bool   `gs:"firewall"`
	Port        uint16 `gs:"port"`
	ProductID   int    `gs:"productid"`
	GameName    string `gs:"gamename"`
	NamespaceID int    `gs:"namespaceid"`
	ID          int    `gs:"id"`
}
type LoginResponse struct {
	Command     int    `gs:"lc"`
	SessKey     int32  `gs:"sesskey"`
	Proof       string `gs:"proof"`
	UserID      uint32 `gs:"userid"`
	ProfileID   uint32 `gs:"profileid"`
	UniqueNick  string `gs:"uniquenick"`
	LoginTicket string `gs:"lt"`
	ID          int    `gs:"id"`
}

func login(state *GameSpySession, req LoginRequest) (LoginResponse, error) {
	if state.LoggedIn {
		logging.Error(state.ModuleName, "Attempt to login twice")
		return LoginResponse{}, ErrLogin
	}

	var authTokenObj common.NASAuthToken
	err := authTokenObj.Unmarshal(req.AuthToken)
	if err != nil {
		logging.Error(state.ModuleName, "Failed to unmarshal auth token:", err)
		if err == common.ErrTokenExpired {
			return LoginResponse{}, ErrLoginLoginTicketExpired
		}

		return LoginResponse{}, ErrLogin
	}

	logging.Info(state.ModuleName, "Game name:", aurora.Cyan(req.GameName))

	state.GameName = req.GameName
	state.GameCode = common.NullTerminatedString(authTokenObj.GameCode[:])
	state.Region = authTokenObj.Region
	state.Language = authTokenObj.Lang
	state.ConsoleFriendCode = authTokenObj.ConsoleFriendCode
	state.UnitCode = authTokenObj.UnitCode

	var endianness binary.ByteOrder = binary.LittleEndian
	if state.UnitCode == UnitCodeWii {
		endianness = binary.BigEndian
	}

	state.InGameName = common.UTF16Decode(authTokenObj.InGameScreenName[:], endianness)

	state.HostPlatform = "DS"
	if state.UnitCode == UnitCodeWii {
		state.HostPlatform = "Wii"
	}

	state.LoginInfoSet = true

	logging.Event(
		"received_login_info",
		map[string]any{
			"wfc_id":       authTokenObj.WFCID,
			"game_name":    state.GameName,
			"wii_number":   state.ConsoleFriendCode,
			"in_game_name": state.InGameName,
			"unit_code":    state.UnitCode,
			"ip_address":   state.RemoteAddr,
		},
	)

	expectedUnitCode := common.GetExpectedUnitCode(state.GameName)
	if (state.UnitCode != UnitCodeDS && state.UnitCode != UnitCodeWii) || (state.UnitCode != expectedUnitCode && expectedUnitCode != UnitCodeDSAndWii) {
		logging.Error(state.ModuleName, "Incorrect unit code specified:", aurora.Cyan(state.UnitCode))
		return LoginResponse{}, ErrLogin
	}

	nasChallenge := common.NullTerminatedString(authTokenObj.Challenge[:])

	response := generateResponse(state.Challenge, nasChallenge, req.AuthToken, req.Challenge)
	if response != req.Response {
		return LoginResponse{}, ErrLogin
	}

	proof := generateProof(state.Challenge, nasChallenge, req.AuthToken, req.Challenge)

	state.Profile, err = db.GetProfile(authTokenObj.ProfileID)
	if err != nil {
		logging.Error(state.ModuleName, "Error getting profile:", err)
		return LoginResponse{}, ErrLogin
	}

	logging.Notice("DATABASE", "Log in GameSpy profile:", aurora.Cyan(authTokenObj.WFCID), "-", aurora.Cyan(authTokenObj.ProfileID))

	state.ModuleName = "GPCM:" + strconv.FormatInt(int64(state.Profile.ID), 10) + "*"
	state.ModuleName += "/" + common.CalcFriendCodeString(state.Profile.ID, state.Profile.GsbrCode[:4]) + "*"

	// Check to see if a session is already open with this profile ID
	mutex.Lock()
	otherSession, exists := sessions[state.Profile.ID]
	if exists {
		otherSession.replyError(ErrForcedDisconnect)

		for i := 0; ; i++ {
			mutex.Unlock()
			time.Sleep(300 * time.Millisecond)
			mutex.Lock()

			_, exists = sessions[state.Profile.ID]
			if !exists {
				break
			}

			// Give up after 6 seconds
			if i >= 20 {
				mutex.Unlock()
				logging.Error(state.ModuleName, "Failed to disconnect other session")
				return LoginResponse{}, ErrForcedDisconnect
			}
		}
	}

	sessions[state.Profile.ID] = state
	mutex.Unlock()

	state.AuthToken = req.AuthToken
	state.LoginTicket = common.GPCMLoginTicket{ProfileID: state.Profile.ID}.Marshal()
	state.SessionKey = rand.Int31n(290000000) + 10000000

	state.LoggedIn = true

	state.ModuleName = "GPCM:" + strconv.FormatInt(int64(state.Profile.ID), 10)
	state.ModuleName += "/" + common.CalcFriendCodeString(state.Profile.ID, state.Profile.GsbrCode[:4])

	logging.Event(
		"logged_in",
		map[string]any{
			"profile_id":   state.Profile.ID,
			"game_name":    state.GameName,
			"in_game_name": state.InGameName,
			"ip_address":   state.RemoteAddr,
		},
	)

	return LoginResponse{
		Command: 2,

		SessKey:     state.SessionKey,
		Proof:       proof,
		UserID:      state.Profile.ID,
		ProfileID:   state.Profile.ID,
		UniqueNick:  state.Profile.UniqueNick(),
		LoginTicket: state.LoginTicket,
		ID:          req.ID,
	}, nil
}

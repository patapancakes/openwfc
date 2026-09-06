package gamestats

import (
	"crypto/md5"
	"encoding/hex"
	"math/rand"
	"owfc/common"
	"owfc/gpcm"
	"owfc/logging"
	"strconv"

	"github.com/logrusorgru/aurora/v3"
)

type AuthRequest struct {
	Command  string `gs:"auth"`
	GameName string `gs:"gamename"`
	Response string `gs:"response"`
	Port     uint16 `gs:"port"`
	ID       int    `gs:"id"`
}

type AuthResponse struct {
	Command int    `gs:"lc"`
	SessKey int32  `gs:"sesskey"`
	Proof   string `gs:"proof"`
	ID      int    `gs:"id"`
}

func auth(state *GameStatsSession, req AuthRequest) (AuthResponse, error) {
	game, ok := common.GetGameInfoByName(req.GameName)
	if !ok {
		return AuthResponse{}, gpcm.ErrDatabase
	}

	var num int32
	for _, b := range []byte(state.Challenge) {
		num = num*-1664117991 + int32(b)
	}

	hash := md5.New()
	hash.Write([]byte(strconv.Itoa(int(num))))
	hash.Write([]byte(game.SecretKey))

	if req.Response != hex.EncodeToString(hash.Sum(nil)) {
		return AuthResponse{}, gpcm.ErrLoginBadPreAuth
	}

	state.SessionKey = rand.Int31n(290000000) + 10000000
	state.GameName = req.GameName
	state.gameInfo = game

	return AuthResponse{
		Command: 2,
		SessKey: state.SessionKey,
		Proof:   "0", // it's like this in a capture
		ID:      req.ID,
	}, nil
}

type AuthProfileRequest struct {
	Command   string `gs:"authp"`
	AuthToken string `gs:"authtoken"`
	Response  string `gs:"resp"`
	LocalID   int    `gs:"lid"`
}

type AuthProfileResponse struct {
	Command uint32 `gs:"pauthr"`
	LocalID int    `gs:"lid"`

	ErrorMessage string `gs:"errmsg,omitzero"`
}

func authProfile(state *GameStatsSession, req AuthProfileRequest) (AuthProfileResponse, error) {
	errMsg := AuthProfileResponse{
		Command:      0,
		LocalID:      req.LocalID,
		ErrorMessage: "Invalid Validation",
	}

	state.LocalID = req.LocalID

	var authTokenObj common.NASAuthToken
	err := authTokenObj.Unmarshal(req.AuthToken)
	if err != nil {
		logging.Error(state.ModuleName, "Error unmarshalling authtoken:", err.Error())
		return errMsg, nil
	}

	state.Profile, err = db.GetProfile(authTokenObj.ProfileID)
	if err != nil {
		logging.Error(state.ModuleName, "Error getting profile:", err.Error())
		return errMsg, nil
	}

	state.ModuleName = "GSTATS:" + strconv.FormatInt(int64(state.Profile.ID), 10)
	state.Authenticated = true

	logging.Notice(state.ModuleName, "Authenticated, game name:", aurora.Cyan(state.gameInfo.Name))

	return AuthProfileResponse{
		Command: state.Profile.ID,
		LocalID: req.LocalID,
	}, nil
}

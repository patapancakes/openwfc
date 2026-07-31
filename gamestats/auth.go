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

func (g *GameStatsSession) auth(command common.GameSpyCommand) {
	game, ok := common.GetGameInfoByName(command.OtherValues["gamename"])
	if !ok {
		g.replyError(gpcm.ErrDatabase)
		return
	}

	var num int32
	for _, b := range []byte(g.Challenge) {
		num = num*-1664117991 + int32(b)
	}

	hash := md5.New()
	hash.Write([]byte(strconv.Itoa(int(num))))
	hash.Write([]byte(game.SecretKey))

	response := command.OtherValues["response"]
	if response != hex.EncodeToString(hash.Sum(nil)) {
		g.replyError(gpcm.ErrLoginBadPreAuth)
		return
	}

	g.SessionKey = rand.Int31n(290000000) + 10000000
	g.GameName = command.OtherValues["gamename"]
	g.gameInfo = game

	g.Write(common.GameSpyCommand{
		Command:      "lc",
		CommandValue: "2",
		OtherValues: map[string]string{
			"sesskey": strconv.FormatInt(int64(g.SessionKey), 10),
			"proof":   "0",
			"id":      "1",
		},
	})
}

func (g *GameStatsSession) authp(command common.GameSpyCommand) {
	lid := command.OtherValues["lid"]
	errorCmd := common.GameSpyCommand{
		Command:      "pauthr",
		CommandValue: "-3",
		OtherValues: map[string]string{
			"lid":    lid,
			"errmsg": "Invalid Validation",
		},
	}

	if lid != "" {
		var err error
		g.LoginID, err = strconv.Atoi(lid)
		if err != nil {
			logging.Error(g.ModuleName, "Error parsing login ID:", err.Error())
			g.Write(errorCmd)
			return
		}
	}

	authToken := command.OtherValues["authtoken"]
	if authToken == "" {
		logging.Error(g.ModuleName, "No authtoken provided")
		g.Write(errorCmd)
		return
	}

	var authTokenObj common.NASAuthToken
	err := authTokenObj.Unmarshal(authToken)
	if err != nil {
		logging.Error(g.ModuleName, "Error unmarshalling authtoken:", err.Error())
		g.Write(errorCmd)
		return
	}

	g.Profile, err = db.GetProfile(authTokenObj.ProfileID)
	if err != nil {
		logging.Error(g.ModuleName, "Error getting profile:", err.Error())
		g.Write(errorCmd)
		return
	}

	g.ModuleName = "GSTATS:" + strconv.FormatInt(int64(g.Profile.ID), 10)
	g.Authenticated = true

	logging.Notice(g.ModuleName, "Authenticated, game name:", aurora.Cyan(g.gameInfo.Name))

	g.Write(common.GameSpyCommand{
		Command:      "pauthr",
		CommandValue: strconv.FormatUint(uint64(g.Profile.ID), 10),
		OtherValues: map[string]string{
			"lid": lid,
		},
	})
}

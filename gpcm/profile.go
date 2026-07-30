package gpcm

import (
	"owfc/common"
	"owfc/database"
	"owfc/logging"
	"strconv"

	"github.com/logrusorgru/aurora/v3"
)

func (g *GameSpySession) getProfile(command common.GameSpyCommand) {
	profileId, err := strconv.Atoi(command.OtherValues["profileid"])
	if err != nil {
		// There was an error getting profile info.
		g.replyError(ErrGetProfile)
		return
	}

	logging.Info(g.ModuleName, "Looking up the profile of", aurora.Cyan(profileId).String())

	var profile database.Profile
	var locstring string

	mutex.Lock()
	if session, ok := sessions[uint32(profileId)]; ok && session.LoggedIn {
		locstring = session.LocString
		profile = session.Profile
		mutex.Unlock()
	} else {
		mutex.Unlock()
		profile, err = db.GetProfile(uint32(profileId))
		if err != nil {
			// The profile info was requested on is invalid.
			g.replyError(ErrGetProfileBadProfile)
			return
		}
	}

	g.WriteBuffer += common.CreateGameSpyMessage(common.GameSpyCommand{
		Command:      "pi",
		CommandValue: "",
		OtherValues: map[string]string{
			"profileid":  command.OtherValues["profileid"],
			"nick":       profile.UniqueNick(),
			"userid":     strconv.FormatUint(uint64(profile.UserID), 10),
			"email":      profile.Email(),
			"sig":        common.RandomHexString(32),
			"uniquenick": profile.UniqueNick(),
			"firstname":  profile.FirstName,
			"lastname":   profile.LastName,
			"pid":        "11",
			"lon":        "0.000000",
			"lat":        "0.000000",
			"loc":        locstring,
			"id":         command.OtherValues["id"],
		},
	})
}

func (g *GameSpySession) updateProfile(command common.GameSpyCommand) {
	firstname, ok := command.OtherValues["firstname"]
	if ok {
		g.Profile.FirstName = firstname
	}

	lastname, ok := command.OtherValues["lastname"]
	if ok {
		g.Profile.LastName = lastname
	}

	db.UpdateProfile(g.Profile)
}

func VerifyPlayerSearch(profileId uint32, sessionKey int32, gameName string) (string, bool) {
	mutex.Lock()
	defer mutex.Unlock()

	if session, ok := sessions[profileId]; ok && session.LoggedIn && session.SessionKey == sessionKey && session.GameName == gameName {
		return "000000000" + session.Profile.GsbrCode[:4] + "0000000", true
	}

	return "", false
}

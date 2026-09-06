package gpcm

import (
	"owfc/common"
	"owfc/common/gamespy"
	"owfc/database"
	"owfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

type GetProfileRequest struct {
	Command   string `gs:"getprofile"`
	SessKey   int32  `gs:"sesskey"`
	ProfileID uint32 `gs:"profileid"`
	ID        int    `gs:"id"`
}

type GetProfileResponse struct {
	Command    string `gs:"pi"`
	ProfileID  uint32 `gs:"profileid"`
	Nick       string `gs:"nick"`
	UserID     uint64 `gs:"userid"`
	Email      string `gs:"email"`
	Signature  string `gs:"sig"`
	UniqueNick string `gs:"uniquenick"`
	FirstName  string `gs:"firstname"`
	LastName   string `gs:"lastname"`
	PartnerID  int    `gs:"pid"`
	Longitute  string `gs:"lon"` // float
	Latitute   string `gs:"lat"` // float
	Location   string `gs:"loc"`
	ID         int    `gs:"id"`
}

func getProfile(state *GameSpySession, req GetProfileRequest) (GetProfileResponse, error) {
	if !state.LoggedIn {
		return GetProfileResponse{}, ErrNotLoggedIn
	}

	logging.Info(state.ModuleName, "Looking up the profile of", aurora.Cyan(req.ProfileID).String())

	var profile database.Profile
	var locstring string

	mutex.Lock()
	if session, ok := sessions[req.ProfileID]; ok && session.LoggedIn {
		mutex.Unlock()

		locstring = session.LocString
		profile = session.Profile
	} else {
		mutex.Unlock()

		var err error
		profile, err = db.GetProfile(req.ProfileID)
		if err != nil {
			return GetProfileResponse{}, ErrGetProfileBadProfile
		}
	}

	return GetProfileResponse{
		ProfileID:  profile.ID,
		Nick:       profile.UniqueNick(),
		UserID:     profile.UserID,
		Email:      profile.Email(),
		Signature:  common.RandomHexString(32),
		UniqueNick: profile.UniqueNick(),
		FirstName:  profile.FirstName,
		LastName:   profile.LastName,
		PartnerID:  11,
		Longitute:  "0.000000",
		Latitute:   "0.000000",
		Location:   locstring,
		ID:         req.ID,
	}, nil
}

type UpdateProfileRequest struct {
	FirstName string `gs:"firstname"`
	LastName  string `gs:"lastname"`
}

func updateProfile(state *GameSpySession, req UpdateProfileRequest) (gamespy.NoResponse, error) {
	if !state.LoggedIn {
		return gamespy.NoResponse{}, ErrNotLoggedIn
	}

	state.Profile.FirstName = req.FirstName
	state.Profile.LastName = req.LastName
	db.UpdateProfile(state.Profile)

	return gamespy.NoResponse{}, nil
}

func VerifySessionKey(profileId uint32, sessionKey int32) bool {
	mutex.Lock()
	defer mutex.Unlock()

	session, ok := sessions[profileId]
	if !ok {
		return false
	}

	if !session.LoggedIn {
		return false
	}

	return session.SessionKey == sessionKey
}

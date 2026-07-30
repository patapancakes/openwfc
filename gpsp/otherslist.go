package gpsp

import (
	"owfc/common"
	"owfc/gpcm"
	"owfc/logging"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

// get list of the specified profiles that have added you as a friend
func handleOthersList(command common.GameSpyCommand) string {
	moduleName := "GPSP"

	profileId, err := strconv.Atoi(command.OtherValues["profileid"])
	if err != nil {
		logging.Error(moduleName, "Invalid profileid:", command.OtherValues["profileid"])
		return gpcm.ErrSearch.GetMessage()
	}

	sessionKey, err := strconv.Atoi(command.OtherValues["sesskey"])
	if err != nil {
		logging.Error(moduleName, "Invalid sesskey:", command.OtherValues["sesskey"])
		return gpcm.ErrSearch.GetMessage()
	}

	if !gpcm.VerifySessionKey(uint32(profileId), int32(sessionKey)) {
		logging.Error(moduleName, "Invalid sesskey:", command.OtherValues["sesskey"])
		return gpcm.ErrSearch.GetMessage()
	}

	moduleName = "GPSP:" + command.OtherValues["profileid"]
	logging.Info(moduleName, "Lookup otherslist for", aurora.Cyan(profileId))

	numopids, err := strconv.Atoi(command.OtherValues["numopids"])
	if err != nil {
		logging.Error(moduleName, "Invalid numopids:", command.OtherValues["numopids"])
		return gpcm.ErrSearch.GetMessage()
	}

	// why even send the request at this point
	if numopids == 0 {
		return `\otherslist\\oldone\\final`
	}

	var opids []uint32
	for opid := range strings.SplitSeq(command.OtherValues["opids"], "|") {
		opidInt, err := strconv.Atoi(opid)
		if err != nil {
			logging.Error("Invalid opid:", opid)
			return gpcm.ErrSearch.GetMessage()
		}

		opids = append(opids, uint32(opidInt))
	}
	if len(opids) != numopids {
		logging.Error(moduleName, "Mismatch opids length with numopids:", aurora.Cyan(len(opids)), "!=", aurora.Cyan(numopids))
		return gpcm.ErrSearch.GetMessage()
	}

	var payload strings.Builder
	payload.WriteString(`\otherslist\`)
	for _, opid := range opids {
		friends, err := db.GetFriends(opid, false)
		if err != nil {
			logging.Error(moduleName, "Failed to get profile friend list:", err)
			return gpcm.ErrSearch.GetMessage()
		}
		for _, friend := range friends {
			if friend.ID != uint32(profileId) {
				continue
			}

			// TODO: see if unauthorized friends should be skipped

			profile, err := db.GetProfile(opid)
			if err != nil {
				logging.Error(moduleName, "Failed to get friend profile:", err)
				return gpcm.ErrSearch.GetMessage()
			}

			payload.WriteString(`\o\` + strconv.Itoa(int(profile.ID)))
			payload.WriteString(`\uniquenick\` + profile.UniqueNick())
		}
	}

	payload.WriteString(`\oldone\\final\`)
	return payload.String()
}

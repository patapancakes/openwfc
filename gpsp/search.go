package gpsp

import (
	"owfc/common"
	"owfc/database"
	"owfc/gpcm"
	"owfc/logging"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

func handleSearch(command common.GameSpyCommand) string {
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

	// TODO: implement more and skip

	moduleName = "GPSP:" + command.OtherValues["profileid"]

	filter := map[string]string{}
	var logInfo strings.Builder
	for _, field := range []string{"nick", "uniquenick", "email", "firstname", "lastname", "icquin" /*"skip"*/} {
		value, ok := command.OtherValues[field]
		if !ok {
			continue
		}

		filter[field] = value
		logInfo.WriteString(" " + aurora.BrightCyan(field).String() + ": '" + aurora.Cyan(value).String() + "'")
	}

	if logInfo.String() == "" {
		logging.Info(moduleName, "Search with no fields")
	} else {
		logging.Info(moduleName, "Search"+logInfo.String())
	}

	ids, err := db.SearchProfile(filter)
	if err != nil {
		logging.Error(moduleName, "Failed to search profiles:", err)
		return gpcm.ErrSearch.GetMessage()
	}

	var profiles []database.Profile
	for _, id := range ids {
		profile, err := db.GetProfile(id)
		if err != nil {
			logging.Error(moduleName, "Failed to get profile", err)
			return gpcm.ErrSearch.GetMessage()
		}

		profiles = append(profiles, profile)
	}

	var payload strings.Builder
	for _, profile := range profiles {
		payload.WriteString(`\bsr\`)
		payload.WriteString(strconv.Itoa(int(profile.ID)))

		// nick

		payload.WriteString(`\uniquenick\`)
		payload.WriteString(profile.UniqueNick())

		// namespaceid

		payload.WriteString(`\firstname\`)
		payload.WriteString(profile.FirstName)

		payload.WriteString(`\lastname\`)
		payload.WriteString(profile.LastName)

		payload.WriteString(`\email\`)
		payload.WriteString(profile.Email())
	}

	payload.WriteString(`\bsrdone\\final\`)

	return payload.String()
}

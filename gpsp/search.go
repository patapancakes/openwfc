package gpsp

import (
	"owfc/database"
	"owfc/gpcm"
	"owfc/logging"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

type SearchRequest struct {
	Command     string `gs:"search"`
	SessKey     int32  `gs:"sesskey"`
	ProfileID   uint32 `gs:"profileid"`
	NamespaceID int    `gs:"namespaceid"`
	PartnerID   int    `gs:"partnerid"`

	Nick       string `gs:"nick"`
	UniqueNick string `gs:"uniquenick"`
	Email      string `gs:"email"`
	FirstName  string `gs:"firstname"`
	LastName   string `gs:"lastname"`
	ICQUIN     int    `gs:"icquin"`

	Skip int `gs:"skip"`
}
type SearchResponse struct {
	Entries []SearchResponseEntry
	Command string `gs:"bsrdone"`
}
type SearchResponseEntry struct {
	ProfileID  uint32 `gs:"bsr"`
	Nick       string `gs:"nick"`
	UniqueNick string `gs:"uniquenick"`
	Email      string `gs:"email"`
	FirstName  string `gs:"firstname"`
	LastName   string `gs:"lastname"`
	//ICQUIN     int    `gs:"icquin"`
}

func search(_ *gpcm.GameSpySession, req SearchRequest) (SearchResponse, error) {
	moduleName := "GPSP"

	if !gpcm.VerifySessionKey(req.ProfileID, req.SessKey) {
		logging.Error(moduleName, "Invalid sesskey:", req.SessKey)
		return SearchResponse{}, gpcm.ErrSearch
	}

	// TODO: implement more and skip

	moduleName = "GPSP:" + strconv.Itoa(int(req.ProfileID))

	filter := map[string]string{}
	var logInfo strings.Builder
	for field, v := range reflect.ValueOf(req).Fields() {
		key := field.Tag.Get("gs")
		if !slices.Contains([]string{"nick", "uniquenick", "email", "firstname", "lastname" /*"icquin" "skip"*/}, key) {
			continue
		}

		filter[key] = v.String()
		logInfo.WriteString(" ")
		logInfo.WriteString(aurora.BrightCyan(key).String())
		logInfo.WriteString(": '")
		logInfo.WriteString(aurora.Cyan(v.String()).String())
		logInfo.WriteString("'")
	}

	if logInfo.String() == "" {
		logging.Info(moduleName, "Search with no fields")
	} else {
		logging.Info(moduleName, "Search"+logInfo.String())
	}

	ids, err := db.SearchProfile(filter)
	if err != nil {
		logging.Error(moduleName, "Failed to search profiles:", err)
		return SearchResponse{}, gpcm.ErrSearch
	}

	var profiles []database.Profile
	for _, id := range ids {
		profile, err := db.GetProfile(id)
		if err != nil {
			logging.Error(moduleName, "Failed to get profile", err)
			return SearchResponse{}, gpcm.ErrSearch
		}

		profiles = append(profiles, profile)
	}

	var entries []SearchResponseEntry
	for _, profile := range profiles {
		entries = append(entries, SearchResponseEntry{
			ProfileID: profile.ID,
			// nick
			UniqueNick: profile.UniqueNick(),
			// namespaceid
			FirstName: profile.FirstName,
			LastName:  profile.LastName,
			Email:     profile.Email(),
		})
	}

	return SearchResponse{Entries: entries}, nil
}

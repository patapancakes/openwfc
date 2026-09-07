package gpsp

import (
	"owfc/gpcm"
	"owfc/logging"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

type OthersListRequest struct {
	Command        string `gs:"otherslist"`
	SessKey        int32  `gs:"sesskey"`
	ProfileID      uint32 `gs:"profileid"`
	OtherPIDsCount int    `gs:"numopids"`
	OtherPIDs      string `gs:"opids"`
}
type OthersListResponse struct {
	Command string `gs:"otherslist"`
	Entries []OthersListResponseEntry
	Done    string `gs:"oldone"`
}
type OthersListResponseEntry struct {
	OtherID    uint32 `gs:"o"`
	UniqueNick string `gs:"uniquenick"`
}

// get list of the specified profiles that have added you as a friend
func othersList(_ *gpcm.GameSpySession, req OthersListRequest) (OthersListResponse, error) {
	moduleName := "GPSP"

	if !gpcm.VerifySessionKey(req.ProfileID, req.SessKey) {
		logging.Error(moduleName, "Invalid sesskey:", req.SessKey)
		return OthersListResponse{}, gpcm.ErrSearch
	}

	moduleName = "GPSP:" + strconv.Itoa(int(req.ProfileID))
	logging.Info(moduleName, "Lookup otherslist for", aurora.Cyan(req.ProfileID))

	// why even send the request at this point
	if req.OtherPIDsCount == 0 {
		return OthersListResponse{}, nil
	}

	var opids []uint32
	for opid := range strings.SplitSeq(req.OtherPIDs, "|") {
		opidInt, err := strconv.Atoi(opid)
		if err != nil {
			logging.Error("Invalid opid:", opid)
			return OthersListResponse{}, gpcm.ErrSearch
		}

		opids = append(opids, uint32(opidInt))
	}
	if len(opids) != req.OtherPIDsCount {
		logging.Error(moduleName, "Mismatch opids length with numopids:", aurora.Cyan(len(opids)), "!=", aurora.Cyan(req.OtherPIDsCount))
		return OthersListResponse{}, gpcm.ErrSearch
	}

	var others []OthersListResponseEntry
	for _, opid := range opids {
		friends, err := db.GetFriends(opid, false)
		if err != nil {
			logging.Error(moduleName, "Failed to get profile friend list:", err)
			return OthersListResponse{}, gpcm.ErrSearch
		}
		for _, friend := range friends {
			if friend.ID != req.ProfileID {
				continue
			}

			// TODO: see if unauthorized friends should be skipped

			profile, err := db.GetProfile(opid)
			if err != nil {
				logging.Error(moduleName, "Failed to get friend profile:", err)
				return OthersListResponse{}, gpcm.ErrSearch
			}

			others = append(others, OthersListResponseEntry{
				OtherID:    profile.ID,
				UniqueNick: profile.UniqueNick(),
			})
		}
	}

	return OthersListResponse{Entries: others}, nil
}

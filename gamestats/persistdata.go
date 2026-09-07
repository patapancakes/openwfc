package gamestats

import (
	"database/sql"
	"owfc/common/gamespy"
	"owfc/gpcm"
	"owfc/logging"
	"strings"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

type GetPersistDataRequest struct {
	Command   string `gs:"getpd"`
	ProfileID uint32 `gs:"pid"`
	DataType  int    `gs:"ptype"`
	DataIndex int    `gs:"dindex"`
	KeyValues bool   `gs:"kv"`
	Keys      string `gs:"keys"`
	LocalID   int    `gs:"lid"`
	Modified  int64  `gs:"mod"`
}
type GetPersistDataResponse struct {
	Command   bool   `gs:"getpdr"` // success
	LocalID   int    `gs:"lid"`
	ProfileID uint32 `gs:"pid"`
	Modified  int64  `gs:"mod"`
	Length    int    `gs:"length"`
	Data      string `gs:"data,raw"`
}

func getPersistData(state *GameStatsSession, req GetPersistDataRequest) (GetPersistDataResponse, error) {
	if !state.Authenticated {
		logging.Error(state.ModuleName, "Attempt to run command before authentication")
		return GetPersistDataResponse{}, gpcm.ErrNotLoggedIn
	}

	errMsg := GetPersistDataResponse{
		Command:   false,
		LocalID:   req.LocalID,
		ProfileID: req.ProfileID,
	}

	if req.DataType > 3 || (req.DataType != PublicReadWrite && req.DataType != PrivateReadWrite) {
		logging.Error(state.ModuleName, "Invalid ptype:", aurora.Cyan(req.DataType))
		return errMsg, nil
	}

	logging.Info(state.ModuleName, "Get persist data: PID:", aurora.Cyan(req.ProfileID), "Type:", aurora.Cyan(req.DataType), "Index:", aurora.Cyan(req.DataIndex))

	if (req.DataType == PrivateRead || req.DataType == PrivateReadWrite) && (req.ProfileID != state.Profile.ID) {
		logging.Warn(state.ModuleName, "Private persist data access attempt for other profile")
		return errMsg, nil
	}

	var data string
	var modified time.Time
	var err error
	if req.KeyValues {
		var kv gamespy.KeyValues
		kv, modified, err = db.GetGameStatsPersistDataKV(req.ProfileID, req.DataType, req.DataIndex, strings.Split(req.Keys, string(byte(1))))

		data = kv.Encode()
	} else {
		data, modified, err = db.GetGameStatsPersistData(req.ProfileID, req.DataType, req.DataIndex)
	}
	if err != nil {
		if err != sql.ErrNoRows {
			logging.Error(state.ModuleName, "GetGameStatsPersistData returned", err)
			return errMsg, nil
		}

		logging.Warn(state.ModuleName, "No data found")
		return errMsg, nil
	}

	if req.Modified != 0 {
		// if modified before mod, return empty success
		if modified.Before(time.Unix(req.Modified, 0)) {
			data = ""
		}
	}

	return GetPersistDataResponse{
		Command:   true,
		ProfileID: req.ProfileID,
		Modified:  modified.Unix(),
		Length:    len(data),
		Data:      data,
	}, nil
}

type SetPersistDataRequest struct {
	Command   string `gs:"setpd"`
	ProfileID uint32 `gs:"pid"`
	DataType  int    `gs:"ptype"`
	DataIndex int    `gs:"dindex"`
	KeyValues bool   `gs:"kv"`
	LocalID   int    `gs:"lid"`
	Length    int    `gs:"length"`
	Data      string `gs:"data,raw"`
}
type SetPersistDataResponse struct {
	Command   bool   `gs:"setpdr"` // success
	LocalID   int    `gs:"lid"`
	ProfileID uint32 `gs:"pid"`
	Modified  int64  `gs:"mod"`
}

func setPersistData(state *GameStatsSession, req SetPersistDataRequest) (SetPersistDataResponse, error) {
	if !state.Authenticated {
		logging.Error(state.ModuleName, "Attempt to run command before authentication")
		return SetPersistDataResponse{}, gpcm.ErrNotLoggedIn
	}

	errMsg := SetPersistDataResponse{
		Command:   false,
		LocalID:   req.LocalID,
		ProfileID: req.ProfileID,
	}

	if req.ProfileID != state.Profile.ID {
		logging.Error(state.ModuleName, "Invalid profile ID:", aurora.Cyan(req.ProfileID))
		return errMsg, nil
	}

	if req.DataType != PublicReadWrite && req.DataType != PrivateReadWrite {
		logging.Error(state.ModuleName, "Invalid ptype:", aurora.Cyan(req.DataType))
		return errMsg, nil
	}

	logging.Info(state.ModuleName, "Set persist data: PID:", aurora.Cyan(state.Profile.ID), "Type:", aurora.Cyan(req.DataType), "Index:", aurora.Cyan(req.DataIndex), "Data:", aurora.Cyan(req.Data))

	// Trim extra null byte at the end
	req.Data = strings.TrimSuffix(req.Data, "\x00")
	if strings.ContainsRune(req.Data, 0) {
		logging.Error(state.ModuleName, "Data contains null byte")
		return errMsg, nil
	}

	var modified time.Time
	var err error
	if req.KeyValues {
		modified, err = db.SetGameStatsPersistDataKV(state.Profile.ID, req.DataType, req.DataIndex, gamespy.KeyValuesFromString(req.Data))
	} else {
		modified, err = db.SetGameStatsPersistData(state.Profile.ID, req.DataType, req.DataIndex, req.Data)
	}
	if err != nil {
		logging.Error(state.ModuleName, "SetGameStatsPersistData returned", err)
		return errMsg, nil
	}

	// TODO: Is mod supposed to be the last modified time or new modified time?
	return SetPersistDataResponse{
		Command:   true,
		LocalID:   req.LocalID,
		ProfileID: req.ProfileID,
		Modified:  modified.Unix(),
	}, nil
}

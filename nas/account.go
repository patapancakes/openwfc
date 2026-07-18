package nas

import (
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"owfc/common"
	"owfc/database"
	"owfc/logging"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

const (
	LoginOK      = "001"
	AcctCreateOK = "002"
	SvcLocOK     = "007"
	LoginProfane = "040"

	Unavailable      = "101"
	UserBanned       = "102"
	UserInfoMismatch = "103"
	UserIDTaken      = "104"
	UserIDUnknown    = "105"
	DeviceIDUsed     = "106"
	MissingParam     = "109"
)

var accountActions = map[string]func(moduleName string, fields map[string][]byte) map[string]string{
	"acctcreate": acctcreate,
	"login":      login,
	"svcloc":     svcloc,
}

func handleAuthAccountEndpoint(w http.ResponseWriter, r *http.Request) {
	moduleName := getModuleName(r)

	fields, err := parseAuthRequest(moduleName, r)
	if err != nil {
		replyHTTPError(w, 400, "400 Bad Request")
		return
	}

	action := string(fields["action"])
	if action == "" {
		logging.Error(moduleName, "No action in form")
		replyHTTPError(w, 400, "400 Bad Request")
		return
	}

	if actionFunc, exists := accountActions[strings.ToLower(action)]; exists {
		reply := actionFunc(moduleName, fields)
		writeAuthResponse(w, reply)
		return
	}

	logging.Error(moduleName, "Unknown action:", aurora.Cyan(action))
	replyHTTPError(w, 400, "400 Bad Request")
}

func acctcreate(moduleName string, fields map[string][]byte) map[string]string {
	param := map[string]string{
		"retry":    "0",
		"datetime": getDateTime(),
		"returncd": AcctCreateOK,
	}

	var user database.User

	var err error
	user.UnitCode, err = strconv.Atoi(string(fields["unitcd"]))
	if err != nil || (user.UnitCode != 0 && user.UnitCode != 1) {
		logging.Error(moduleName, "Invalid unitcd string in form")
		param["returncd"] = MissingParam
		return param
	}

	user.ID, err = strconv.ParseUint(string(fields["userid"]), 10, 64)
	if err != nil {
		logging.Error(moduleName, "Invalid userid string in form")
		param["returncd"] = MissingParam
		return param
	}

	macadr, ok := fields["macadr"]
	if !ok || len(macadr) != 12 {
		logging.Error(moduleName, "Invalid macadr string in form")
		param["returncd"] = MissingParam
		return param
	}
	user.MacAddress = string(macadr)

	if !user.IsWii() {
		_, mac, _, _ := decodeDSUserID(user.ID)
		if !strings.HasSuffix(user.MacAddress, strconv.FormatUint(uint64(mac), 16)) {
			logging.Error(moduleName, "MAC does not match userid")
			param["returncd"] = UserInfoMismatch
			return param
		}

		user.Password, err = strconv.Atoi(string(fields["passwd"]))
		if err != nil || user.Password > 999 {
			logging.Error(moduleName, "Invalid passwd string in form")
			param["returncd"] = MissingParam
			return param
		}
	} else {
		csnum, ok := fields["csnum"]
		if !ok || len(csnum) != 11 {
			logging.Error(moduleName, "Invalid csnum string in form")
			param["returncd"] = MissingParam
			return param
		}
		user.SerialNumber = string(csnum)
	}

	err = db.CreateUser(user)
	if err != nil {
		logging.Error(moduleName, "Error creating user:", aurora.Cyan(user.ID), "\nerror:", err.Error())
		switch err {
		case database.ErrUserIDInUse:
			param["returncd"] = UserIDTaken
		case database.ErrMACInUse, database.ErrSerialNumberInUse:
			param["returncd"] = DeviceIDUsed
		default:
			param["returncd"] = Unavailable
		}

		return param
	}

	logging.Notice(moduleName, "Created new NAS user:", aurora.Cyan(user.ID), aurora.Cyan(user.MacAddress))

	return param
}

func login(moduleName string, fields map[string][]byte) map[string]string {
	param := map[string]string{
		"retry":    "0",
		"datetime": getDateTime(),
		"returncd": LoginOK,
		"locator":  "gamespy.com",
	}

	var token common.NASAuthToken

	var err error
	token.UserID, err = strconv.ParseUint(string(fields["userid"]), 10, 64)
	if err != nil || token.UserID >= 0x80000000000 {
		logging.Error(moduleName, "Invalid userid string in form")
		param["returncd"] = MissingParam
		return param
	}

	user, ok := db.GetUser(token.UserID)
	if !ok {
		/*logging.Error(moduleName, "Unknown userid")
		param["returncd"] = UserIDUnknown
		return param*/

		// conntest will acctcreate with NAS before DNS can be set
		// try to create the account now if it doesn't exist
		// TODO: add a config value for this
		createResponse := acctcreate(moduleName, fields)
		if createResponse["returncd"] != AcctCreateOK {
			param["returncd"] = createResponse["returncd"]
			return param
		}
	}
	if user.Banned {
		logging.Error(moduleName, "User is banned")
		param["returncd"] = UserBanned
		return param
	}

	gamecd, ok := fields["gamecd"]
	if !ok || len(gamecd) != 4 {
		logging.Error(moduleName, "Invalid gamecd in form")
		param["returncd"] = MissingParam
		return param
	}
	copy(token.GameCode[:], gamecd)

	gsbrcd := string(fields["gsbrcd"])
	if gsbrcd != "" {
		if len(gsbrcd) != 11 || !strings.HasPrefix(gsbrcd, string(gamecd[:3])) {
			// some games don't set the region in gsbrcd correctly (mkds)
			logging.Error(moduleName, "Invalid gsbrcd string in form")
			param["returncd"] = MissingParam
			return param
		}

		// acctcreate creates a GameSpy user, and the client logs into an existing profile on GPCM
		// login probably is what created the profile
		token.ProfileID, err = db.GetProfileID(user.ID, gsbrcd)
		if err != nil {
			if err != sql.ErrNoRows {
				logging.Error(moduleName, "Error getting GameSpy profile ID:", aurora.Cyan(user.ID), aurora.Cyan(gsbrcd), "\nerror:", err.Error())
				param["returncd"] = Unavailable
				return param
			}

			token.ProfileID, err = db.CreateProfile(user.ID, gsbrcd)
			if err != nil {
				logging.Error(moduleName, "Error creating GameSpy profile:", aurora.Cyan(user.ID), aurora.Cyan(gsbrcd), "\nerror:", err.Error())
				param["returncd"] = Unavailable
				return param
			}

			logging.Notice(moduleName, "Created new GameSpy profile:", aurora.Cyan(user.ID), aurora.Cyan(gsbrcd), aurora.Cyan(token.ProfileID))

			logging.Event(
				"profile_created",
				map[string]any{
					"user_id":    user.ID,
					"profile_id": token.ProfileID,
					"gsbrcd":     gsbrcd,
				},
			)
		}

		challenge := common.RandomString(8)
		copy(token.Challenge[:], []byte(challenge))
		param["challenge"] = challenge
	}

	lang, err := hex.DecodeString(string(fields["lang"]))
	if err != nil || len(lang) != 1 {
		logging.Error(moduleName, "Invalid lang byte in form")
		param["returncd"] = MissingParam
		return param
	}
	token.Lang = lang[0]

	if strconv.Itoa(user.UnitCode) != string(fields["unitcd"]) {
		logging.Error(moduleName, "unitcd does not match")
		param["returncd"] = UserInfoMismatch
		return param
	}
	token.UnitCode = byte(user.UnitCode)

	var endianness binary.ByteOrder = binary.LittleEndian
	if user.IsWii() {
		endianness = binary.BigEndian
	}

	ingamesn, hasInGameSN := fields["ingamesn"]
	if hasInGameSN {
		profane, _ := IsBadWord(common.UTF16Decode(ingamesn, endianness))
		if profane {
			logging.Info(moduleName, "Provided in-game screen name has a profane word:", aurora.Red(common.UTF16Decode(ingamesn, endianness)))
			param["returncd"] = LoginProfane
		}
	}

	if !user.IsWii() {
		if fmt.Sprintf("%03d", user.Password) != string(fields["passwd"]) {
			logging.Error(moduleName, "passwd does not match")
			param["returncd"] = UserIDUnknown
			return param
		}

		devname, ok := fields["devname"]
		if !ok {
			logging.Error(moduleName, "No devname in form")
			param["returncd"] = MissingParam
			return param
		}

		// Only later DS games send ingamesn
		if !hasInGameSN {
			ingamesn = devname
		}
	} else {
		if user.SerialNumber != string(fields["csnum"]) {
			logging.Error(moduleName, "csnum does not match")
			param["returncd"] = UserInfoMismatch
			return param
		}

		token.ConsoleFriendCode, err = strconv.ParseUint(string(fields["cfc"]), 10, 64)
		if err != nil || token.ConsoleFriendCode > 9999999999999999 {
			logging.Error(moduleName, "Invalid cfc string in form")
			param["returncd"] = MissingParam
			return param
		}

		region, err := hex.DecodeString(string(fields["region"]))
		if err != nil || len(region) != 1 {
			logging.Error(moduleName, "Invalid region byte in form")
			param["returncd"] = MissingParam
			return param
		}
		token.Region = region[0]
	}
	copy(token.InGameScreenName[:], ingamesn)

	db.UpdateUserName(user.ID, common.UTF16Decode(ingamesn, endianness))

	console := "(DS)"
	if user.IsWii() {
		console = "(Wii)"
	}

	param["token"] = token.Marshal()

	logging.Notice(moduleName, "Login", console, aurora.Cyan(token.UserID), aurora.Cyan(string(gsbrcd)), "name:", aurora.Cyan(common.UTF16Decode(ingamesn, endianness)))

	return param
}

func svcloc(moduleName string, fields map[string][]byte) map[string]string {
	param := map[string]string{
		"retry":      "0",
		"datetime":   getDateTime(),
		"returncd":   SvcLocOK,
		"statusdata": "Y",
	}

	loginResponse := login(moduleName, fields)
	if loginResponse["returncd"] != LoginOK {
		param["returncd"] = loginResponse["returncd"]
		return param
	}

	switch string(fields["svc"]) {
	default:
		param["svchost"] = "n/a"

	case "9000", "9001":
		param["servicetoken"] = loginResponse["token"]
		param["svchost"] = "dls1.nintendowifi.net"
	}

	return param
}

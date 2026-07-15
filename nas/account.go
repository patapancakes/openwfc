package nas

import (
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"wwfc/common"
	"wwfc/database"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
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
		"returncd": "002",
	}

	var user database.User

	var err error
	user.UnitCode, err = strconv.Atoi(string(fields["unitcd"]))
	if err != nil || (user.UnitCode != 0 && user.UnitCode != 1) {
		logging.Error(moduleName, "Invalid unitcd string in form")
		param["returncd"] = "103"
		return param
	}

	user.ID, err = strconv.ParseUint(string(fields["userid"]), 10, 64)
	if err != nil {
		logging.Error(moduleName, "Invalid userid string in form")
		param["returncd"] = "103"
		return param
	}

	macadr, ok := fields["macadr"]
	if !ok || len(macadr) != 12 {
		logging.Error(moduleName, "Invalid macadr string in form")
		param["returncd"] = "103"
		return param
	}

	user.MacAddress = string(macadr)

	if !user.IsWii() {
		user.Password, err = strconv.Atoi(string(fields["passwd"]))
		if err != nil || user.Password > 999 {
			logging.Error(moduleName, "Invalid passwd string in form")
			param["returncd"] = "103"
			return param
		}
	} else {
		csnum, ok := fields["csnum"]
		if !ok || len(csnum) != 11 {
			logging.Error(moduleName, "Invalid csnum string in form")
			param["returncd"] = "103"
			return param
		}

		user.SerialNumber = string(csnum)
	}

	err = db.CreateUser(&user)
	if err != nil {
		logging.Error(moduleName, "Error creating user:", aurora.Cyan(user.ID), "\nerror:", err.Error())
		switch err {
		case database.ErrUserIDInUse:
			param["returncd"] = "104"
		case database.ErrMACInUse, database.ErrSerialNumberInUse:
			param["returncd"] = "106"
		default:
			param["returncd"] = "103"
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
		"locator":  "gamespy.com",
	}

	var token common.NASAuthToken

	gamecd, ok := fields["gamecd"]
	if !ok || len(gamecd) != 4 {
		logging.Error(moduleName, "Invalid gamecd in form")
		param["returncd"] = "103"
		return param
	}
	copy(token.GameCode[:], gamecd)

	var err error
	token.UserID, err = strconv.ParseUint(string(fields["userid"]), 10, 64)
	if err != nil || token.UserID >= 0x80000000000 {
		logging.Error(moduleName, "Invalid userid string in form")
		param["returncd"] = "103"
		return param
	}

	user, ok := db.GetUser(token.UserID)
	if !ok {
		logging.Error(moduleName, "Unknown userid")
		param["returncd"] = "105"
		return param
	}

	macadr, ok := fields["macadr"]
	if !ok || len(macadr) != 12 {
		logging.Error(moduleName, "Invalid macadr string in form")
		param["returncd"] = "103"
		return param
	}

	gsbrcd, ok := fields["gsbrcd"]
	if !ok {
		logging.Error(moduleName, "No gsbrcd in form")
		param["returncd"] = "103"
		return param
	}

	if (len(gsbrcd) < 4 && len(gsbrcd) != 0) || strings.ContainsRune(string(gsbrcd), 0) {
		logging.Error(moduleName, "Invalid gsbrcd string in form")
		param["returncd"] = "103"
		return param
	}

	// Some games like Fortune Street make login requests without a gsbr code, so we temporarily fake one
	if len(gsbrcd) == 0 {
		gsbrcd = append(gamecd[:3], 'J')
	}

	copy(token.GsbrCode[:], gsbrcd)

	lang, ok := fields["lang"]
	if !ok {
		lang = []byte("ff")
	}

	langByte, err := hex.DecodeString(string(lang))
	if err != nil || len(langByte) != 1 {
		logging.Error(moduleName, "Invalid lang byte in form")
		param["returncd"] = "103"
		return param
	}
	token.Lang = langByte[0]

	unitcd, err := strconv.Atoi(string(fields["unitcd"]))
	if err != nil || (unitcd != 0 && unitcd != 1) {
		logging.Error(moduleName, "Invalid unitcd string in form")
		param["returncd"] = "103"
		return param
	}

	if user.UnitCode != unitcd {
		logging.Error(moduleName, "unitcd does not match")
		param["returncd"] = "103"
		return param
	}

	token.UnitCode = byte(user.UnitCode)

	var endianness binary.ByteOrder = binary.LittleEndian
	if user.IsWii() {
		endianness = binary.BigEndian
	}

	hasProfaneName := false
	ingamesn, hasIngamesn := fields["ingamesn"]
	ingamesnStr := ""
	if hasIngamesn {
		ingamesnStr = common.UTF16Decode(ingamesn, endianness)
		if hasProfaneName, _ = IsBadWord(ingamesnStr); hasProfaneName {
			logging.Info(moduleName, "Provided in-game screen name has a profane word:", aurora.Red(ingamesnStr).String())
			// Continue with different return code
		}
	}

	if !user.IsWii() {
		passwdStr, ok := fields["passwd"]
		if !ok || len(passwdStr) != 3 {
			logging.Error(moduleName, "Invalid passwd string in form")
			param["returncd"] = "103"
			return param
		}

		passwd, err := strconv.Atoi(string(passwdStr))
		if err != nil {
			logging.Error(moduleName, "Invalid passwd string in form")
			param["returncd"] = "103"
			return param
		}

		if user.Password != passwd {
			logging.Error(moduleName, "passwd does not match")
			param["returncd"] = "105"
			return param
		}

		devname, ok := fields["devname"]
		if !ok {
			logging.Error(moduleName, "No devname in form")
			param["returncd"] = "103"
			return param
		}

		// Only later DS games send ingamesn
		if !hasIngamesn {
			ingamesn = devname
		}
		logging.Notice(moduleName, "Login (DS)", aurora.Cyan(token.UserID), aurora.Cyan(string(gsbrcd)), "devname:", aurora.Cyan(common.UTF16Decode(devname, endianness)), "name:", aurora.Cyan(ingamesnStr))
	} else {
		if user.MacAddress != string(macadr) {
			logging.Error(moduleName, "macadr does not match")
			param["returncd"] = "103"
			return param
		}

		csnum, ok := fields["csnum"]
		if !ok || len(csnum) != 11 {
			logging.Error(moduleName, "Invalid csnum string in form")
			param["returncd"] = "103"
			return param
		}

		if user.SerialNumber != string(csnum) {
			logging.Error(moduleName, "csnum does not match")
			param["returncd"] = "105"
			return param
		}

		token.ConsoleFriendCode, err = strconv.ParseUint(string(fields["cfc"]), 10, 64)
		if err != nil || token.ConsoleFriendCode > 9999999999999999 {
			logging.Error(moduleName, "Invalid cfc string in form")
			param["returncd"] = "103"
			return param
		}

		region, ok := fields["region"]
		if !ok {
			region = []byte("ff")
		}

		regionByte, err := hex.DecodeString(string(region))
		if err != nil || len(regionByte) != 1 {
			logging.Error(moduleName, "Invalid region byte in form")
			param["returncd"] = "103"
			return param
		}
		token.Region = regionByte[0]

		logging.Notice(moduleName, "Login (Wii)", aurora.Cyan(token.UserID), aurora.Cyan(string(gsbrcd)), "name:", aurora.Cyan(ingamesnStr))
	}

	challenge := common.RandomString(8)
	copy(token.Challenge[:], []byte(challenge))
	copy(token.InGameScreenName[:], ingamesn)

	param["returncd"] = "001"
	if hasProfaneName {
		param["returncd"] = "040"
	}

	param["challenge"] = challenge
	param["token"] = token.Marshal()

	return param
}

func svcloc(moduleName string, fields map[string][]byte) map[string]string {
	param := map[string]string{
		"retry":      "0",
		"datetime":   getDateTime(),
		"returncd":   "007",
		"statusdata": "Y",
	}

	// TODO: use gamecd as gsbrcd so login works, make new login func maybe?
	fields["gsbrcd"] = fields["gamecd"]

	authToken := login(moduleName, fields)["token"]

	switch string(fields["svc"]) {
	default:
		param["servicetoken"] = authToken
		param["svchost"] = "n/a"

	case "9000", "9001":
		param["servicetoken"] = authToken
		param["svchost"] = "dls1.nintendowifi.net"
	}

	return param
}

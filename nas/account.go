package nas

import (
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
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

var accountHandlers = map[string]AccountHandlerFunc{
	"ACCTCREATE": handleAccount(acAccountCreate),
	"LOGIN":      handleAccount(acLogin),
	"SVCLOC":     handleAccount(acServiceLocator),
}

type AccountHandlerFunc func(moduleName string, req url.Values) (string, error)

func handleAccount[reqT, respT any](handler func(string, reqT) (respT, error)) AccountHandlerFunc {
	return func(moduleName string, req url.Values) (string, error) {
		in, err := Unmarshal[reqT](req)
		if err != nil {
			return "", err
		}

		resp, err := handler(moduleName, in)
		return Marshal(resp), err
	}
}

func handleAuthAccountEndpoint(w http.ResponseWriter, r *http.Request) {
	moduleName := getModuleName(r)

	action, err := common.Base64DwcEncoding.DecodeString(r.FormValue("action"))
	if err != nil {
		logging.Error(moduleName, "No action in form")
		replyHTTPError(w, 400, "400 Bad Request")
	}

	handler, ok := accountHandlers[string(action)]
	if !ok {
		logging.Error(moduleName, "Unknown action:", aurora.Cyan(string(action)))
		replyHTTPError(w, 400, "400 Bad Request")
	}

	resp, err := handler(moduleName, r.PostForm)
	if err != nil {
		logging.Error(moduleName, "Action", aurora.Cyan(string(action)), "returned error:", aurora.Cyan(err))
	}

	w.Write([]byte(resp))
}

type NASRequest struct {
	UserID     uint64 `nas:"userid"`
	GameCode   string `nas:"gamecd"`
	UnitCode   int    `nas:"unitcd"`
	MacAddress string `nas:"macadr"`
	Language   string `nas:"lang"`

	InGameSN string `nas:"ingamesn"` // UTF-16 encoded, not always present

	// ds only
	Password   int    `nas:"passwd"`
	DeviceName string `nas:"devname"`

	// wii only
	SerialNumber string `nas:"csnum"`
	FriendCode   uint64 `nas:"cfc"`
	Region       string `nas:"region"`
}
type NASResponse struct {
	Retry      bool   `nas:"retry"`
	DateTime   string `nas:"datetime"`
	ReturnCode string `nas:"returncd"`
}

type AccountCreateRequest struct {
	NASRequest
}
type AccountCreateResponse struct {
	NASResponse
}

func acAccountCreate(moduleName string, req AccountCreateRequest) (AccountCreateResponse, error) {
	resp := AccountCreateResponse{NASResponse: NASResponse{
		DateTime:   getDateTime(),
		ReturnCode: AcctCreateOK,
	}}

	var user database.User

	user.UnitCode = req.UnitCode
	if user.UnitCode != 0 && user.UnitCode != 1 {
		resp.ReturnCode = MissingParam
		return resp, fmt.Errorf("invalid unitcd")
	}

	user.ID = req.UserID

	user.MacAddress = req.MacAddress
	if len(user.MacAddress) != 12 {
		resp.ReturnCode = MissingParam
		return resp, fmt.Errorf("invalid macadr")
	}

	if !user.IsWii() {
		_, mac, _, _ := decodeDSUserID(user.ID)
		if !strings.HasSuffix(user.MacAddress, strconv.FormatUint(uint64(mac), 16)) {
			resp.ReturnCode = UserInfoMismatch
			return resp, fmt.Errorf("mac does not match userid")
		}

		user.Password = req.Password
		if user.Password > 999 {
			resp.ReturnCode = MissingParam
			return resp, fmt.Errorf("invalid passwd")
		}
	} else {
		user.SerialNumber = req.SerialNumber
		if len(user.SerialNumber) != 11 {
			resp.ReturnCode = MissingParam
			return resp, fmt.Errorf("invalid csnum")
		}
	}

	err := db.CreateUser(user)
	if err != nil {
		switch err {
		case database.ErrUserIDInUse:
			resp.ReturnCode = UserIDTaken
		case database.ErrMACInUse, database.ErrSerialNumberInUse:
			resp.ReturnCode = DeviceIDUsed
		default:
			resp.ReturnCode = Unavailable
		}

		return resp, err
	}

	logging.Notice(moduleName, "Created new NAS user:", aurora.Cyan(user.ID), aurora.Cyan(user.MacAddress))

	return resp, nil
}

type LoginRequest struct {
	NASRequest
	GameSpyCode string `nas:"gsbrcd"`
}
type LoginResponse struct {
	NASResponse
	Locator   string `nas:"locator"`
	Challenge string `nas:"challenge"`
	Token     string `nas:"token"`
}

func acLogin(moduleName string, req LoginRequest) (LoginResponse, error) {
	resp := LoginResponse{
		NASResponse: NASResponse{
			DateTime:   getDateTime(),
			ReturnCode: LoginOK,
		},
		Locator: "gamespy.com",
	}

	var token common.NASAuthToken

	token.WFCID = req.UserID

	user, ok := db.GetUser(token.WFCID)
	if !ok {
		// create account if it doesn't exist
		// TODO: add config value for this
		createResp, err := acAccountCreate(moduleName, AccountCreateRequest{NASRequest: req.NASRequest})
		if err != nil {
			return LoginResponse{NASResponse: createResp.NASResponse}, err
		}

		user, ok = db.GetUser(token.WFCID)
		if !ok {
			resp.ReturnCode = UserIDUnknown
			return resp, fmt.Errorf("unknown userid")
		}
	}
	if user.Banned {
		resp.ReturnCode = UserBanned
		return resp, fmt.Errorf("user is banned")
	}

	copy(token.GameCode[:], []byte(req.GameCode))

	if req.GameSpyCode != "" {
		if len(req.GameSpyCode) != 11 {
			resp.ReturnCode = MissingParam
			return resp, fmt.Errorf("invalid gsbrcd")
		}

		// acctcreate creates a GameSpy user, and the client logs into an existing profile on GPCM
		// login probably is what created the profile
		var err error
		token.ProfileID, err = db.GetProfileID(user.ID, req.GameSpyCode)
		if err != nil {
			if err != sql.ErrNoRows {
				resp.ReturnCode = Unavailable
				return resp, err
			}

			token.ProfileID, err = db.CreateProfile(user.ID, req.GameSpyCode)
			if err != nil {
				resp.ReturnCode = Unavailable
				return resp, err
			}

			logging.Notice(moduleName, "Created new GameSpy profile:", aurora.Cyan(user.ID), aurora.Cyan(req.GameSpyCode), aurora.Cyan(token.ProfileID))

			logging.Event(
				"profile_created",
				map[string]any{
					"user_id":    user.ID,
					"profile_id": token.ProfileID,
					"gsbrcd":     req.GameSpyCode,
				},
			)
		}

		challenge := common.RandomString(8)
		copy(token.Challenge[:], []byte(challenge))
		resp.Challenge = challenge
	}

	lang, err := hex.DecodeString(req.Language)
	if err != nil || len(lang) != 1 {
		resp.ReturnCode = MissingParam
		return resp, fmt.Errorf("invalid lang")
	}
	token.Lang = lang[0]

	if req.UnitCode != user.UnitCode {
		resp.ReturnCode = UserInfoMismatch
		return resp, fmt.Errorf("unitcd does not match")
	}
	token.UnitCode = byte(user.UnitCode)

	var endianness binary.ByteOrder = binary.LittleEndian
	if user.IsWii() {
		endianness = binary.BigEndian
	}

	var name string
	if req.InGameSN != "" {
		name = common.UTF16Decode([]byte(req.InGameSN), endianness)
		profane, _ := IsBadWord(name)
		if profane {
			logging.Info(moduleName, "Provided in-game screen name has a profane word:", aurora.Red(name))
			resp.ReturnCode = LoginProfane
		}
	}

	if !user.IsWii() {
		if req.Password != user.Password {
			resp.ReturnCode = UserIDUnknown
			return resp, fmt.Errorf("passwd does not match")
		}

		if req.DeviceName == "" {
			resp.ReturnCode = MissingParam
			return resp, fmt.Errorf("invalid devname")
		}

		// Only later DS games send ingamesn
		if req.InGameSN == "" {
			name = common.UTF16Decode([]byte(req.DeviceName), binary.LittleEndian)
		}
	} else {
		if user.SerialNumber != req.SerialNumber {
			resp.ReturnCode = UserInfoMismatch
			return resp, fmt.Errorf("csnum does not match")
		}

		token.ConsoleFriendCode = req.FriendCode

		region, err := hex.DecodeString(req.Region)
		if err != nil || len(region) != 1 {
			resp.ReturnCode = MissingParam
			return resp, fmt.Errorf("invalid region")
		}
		token.Region = region[0]
	}
	copy(token.InGameScreenName[:], name)

	db.UpdateUserName(user.ID, name)

	console := "(DS)"
	if user.IsWii() {
		console = "(Wii)"
	}

	resp.Token = token.Marshal()

	logging.Notice(moduleName, "Login", console, aurora.Cyan(token.WFCID), aurora.Cyan(req.GameSpyCode), "name:", aurora.Cyan(name))

	return resp, nil
}

type ServiceLocatorRequest struct {
	NASRequest
	Service int `nas:"svc"`
}
type ServiceLocatorResponse struct {
	NASResponse
	StatusData   string `nas:"statusdata"`
	ServiceHost  string `nas:"svchost"`
	ServiceToken string `nas:"servicetoken"`
}

func acServiceLocator(moduleName string, req ServiceLocatorRequest) (ServiceLocatorResponse, error) {
	resp := ServiceLocatorResponse{
		NASResponse: NASResponse{
			DateTime:   getDateTime(),
			ReturnCode: SvcLocOK,
		},
		StatusData: "Y",
	}

	loginResp, err := acLogin(moduleName, LoginRequest{NASRequest: req.NASRequest})
	if err != nil || loginResp.ReturnCode != LoginOK {
		resp.ReturnCode = loginResp.ReturnCode
		return resp, err
	}

	switch req.Service {
	default:
		resp.ServiceHost = "n/a"
	case 9000, 9001:
		resp.ServiceHost = "dls1.nintendowifi.net"
		resp.ServiceToken = loginResp.Token
	}

	return resp, nil
}

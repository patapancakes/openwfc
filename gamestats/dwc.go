package gamestats

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"owfc/common"
	"owfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

func handleDwcGet(r *http.Request, game common.GameInfo, moduleName string) []byte {
	challenge := makeDwcChallenge(r.PathValue("gamename"), r.PathValue("endpoint"), r.FormValue("pid"))
	if r.FormValue("hash") == "" {
		return []byte(challenge)
	}
	if !verifyDwcHash(game.Stats.Key, challenge, r.FormValue("hash")) {
		logging.Warn(moduleName, "Invalid hash")
		return nil
	}

	// TODO
	data, err := base64.URLEncoding.DecodeString(r.FormValue("data"))
	if err != nil {
		logging.Error(moduleName, "Invalid data:", err)
		return nil
	}

	data = decryptDwc(game.Stats, data)

	logging.Info(moduleName, "DWC Rankings Get with body", aurora.Cyan(hex.EncodeToString(data)))

	response := binary.LittleEndian.AppendUint32([]byte{}, 1) // RNK_GET
	response = binary.LittleEndian.AppendUint32(response, 0)  // count

	return append(response, []byte(makeDwcProof(game.Stats.Key, response))...)
}

func handleDwcPut(r *http.Request, game common.GameInfo, moduleName string) []byte {
	challenge := makeDwcChallenge(r.PathValue("gamename"), r.PathValue("endpoint"), r.FormValue("pid"))
	if r.FormValue("hash") == "" {
		return []byte(challenge)
	}
	if !verifyDwcHash(game.Stats.Key, challenge, r.FormValue("hash")) {
		logging.Warn(moduleName, "Invalid hash")
		return nil
	}

	// TODO
	data, err := base64.URLEncoding.DecodeString(r.FormValue("data"))
	if err != nil {
		logging.Error(moduleName, "Invalid data:", err)
		return nil
	}

	data = decryptDwc(game.Stats, data)

	logging.Info(moduleName, "DWC Rankings Put with body", aurora.Cyan(hex.EncodeToString(data)))

	response := []byte("done")

	return append(response, []byte(makeDwcProof(game.Stats.Key, response))...)
}

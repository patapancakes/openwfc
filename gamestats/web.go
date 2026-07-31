package gamestats

import (
	"net/http"
	"owfc/common"
	"owfc/logging"
	"strconv"

	"github.com/logrusorgru/aurora/v3"
)

func HandleWebRequest(w http.ResponseWriter, r *http.Request) {
	logging.Info("GSTATS", aurora.Yellow(r.Method), aurora.Cyan(r.URL), "via", aurora.Cyan(r.Host), "from", aurora.BrightCyan(r.RemoteAddr))
	moduleName := "GSTATS:" + r.RemoteAddr

	game, ok := common.GetGameInfoByName(r.PathValue("gamename"))
	if !ok {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	var response []byte
	switch r.PathValue("endpoint") {
	// DWC Rankings SDK endpoints
	case "web/client/get.asp", "web/client/get2.asp":
		response = handleDwcGet(r, game, moduleName)
	case "web/client/put.asp", "web/client/put2.asp":
		response = handleDwcPut(r, game, moduleName)

	default:
		logging.Warn(moduleName, "Unhandled path:", aurora.Cyan(r.PathValue("endpoint")))
	}

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Server", "Microsoft-IIS/6.0")
	w.Header().Add("Server", "GSTPRDSTATSWEB2")
	w.Header().Set("X-Powered-By", "ASP.NET")
	w.Header().Set("Content-Length", strconv.Itoa(len(response)))
	w.Write(response)
}

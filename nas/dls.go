package nas

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

var (
	dlsActions = map[string]func(moduleName string, fields map[string][]byte) []byte{
		"count":    dlsCount,
		"list":     dlsList,
		"contents": dlsContents,
	}

	dlcDir = "./dlc"
)

func handleDownloadEndpoint(w http.ResponseWriter, r *http.Request) {
	moduleName := "DLS:" + r.RemoteAddr

	fields, err := parseAuthRequest(moduleName, r)
	if err != nil {
		replyHTTPError(w, 400, "400 Bad Request")
		return
	}

	authToken := string(fields["token"])
	if authToken == "" {
		logging.Error(moduleName, "Missing or invalid token")
		replyHTTPError(w, 400, "400 Bad Request")
		return
	}

	authTokenObj := common.NASAuthToken{}
	err = authTokenObj.Unmarshal(string(authToken))
	if err != nil {
		logging.Error(moduleName, "Failed to unmarshal auth token:", err)
		replyHTTPError(w, 400, "400 Bad Request")
		return
	}

	action := string(fields["action"])
	if action == "" {
		logging.Error(moduleName, "No action in form")
		replyHTTPError(w, 400, "400 Bad Request")
		return
	}

	rhgamecd, ok := fields["rhgamecd"]
	if !ok || !isValidRHGameCode(string(rhgamecd)) {
		logging.Error(moduleName, "Missing or invalid rhgamecd")
		replyHTTPError(w, 400, "400 Bad Request")
		return
	}

	if actionFunc, exists := dlsActions[strings.ToLower(action)]; exists {
		reply := actionFunc(moduleName, fields)

		// TODO: return error from handlers maybe?
		if strings.ToLower(action) == "contents" && reply == nil {
			replyHTTPError(w, 404, "404 Not Found")
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", strconv.Itoa(len(reply)))

		// TODO: fix crazy edge case, sending WH050_100_2011-03-07 in whole
		// causes a slice bounds out of range runtime error
		const chunkLen = 1024 * 4
		for i := 0; i < len(reply); i += chunkLen {
			_, err := w.Write(reply[i:min(i+chunkLen, len(reply))])
			if err != nil {
				logging.Error(moduleName, "Error writing response:", err)
				break
			}
		}
		return
	}

	logging.Error(moduleName, "Unknown action:", aurora.Cyan(action))
	replyHTTPError(w, 400, "400 Bad Request")
}

func dlsCount(moduleName string, fields map[string][]byte) []byte {
	list, err := getDlsList(string(fields["rhgamecd"]))
	if err != nil {
		logging.Error("Unknown game:", aurora.Cyan(fields["rhgamecd"]))
		return []byte{'0'}
	}

	list = filterDlsList(list, fields)

	return []byte(strconv.Itoa(len(list)))
}

func dlsList(moduleName string, fields map[string][]byte) []byte {
	list, err := getDlsList(string(fields["rhgamecd"]))
	if err != nil {
		logging.Error("Unknown game:", aurora.Cyan(fields["rhgamecd"]))
		return nil
	}

	list = filterDlsList(list, fields)

	offset, ok := fields["offset"]
	if ok {
		n, err := strconv.Atoi(string(offset))
		if err != nil {
			return nil
		}
		if n < 0 || n > len(list) {
			return nil
		}

		list = list[n:]
	}

	num, ok := fields["num"]
	if ok {
		n, err := strconv.Atoi(string(num))
		if err != nil {
			return nil
		}
		if n < 0 || n > len(list) {
			return nil
		}

		list = list[:n]
	}

	buf := new(bytes.Buffer)
	cw := csv.NewWriter(buf)
	cw.Comma = '\t'
	cw.UseCRLF = true

	err = cw.WriteAll(list)
	if err != nil {
		return nil
	}

	buf.WriteString("\r\n")

	return buf.Bytes()
}

func getDlsList(rhgamecd string) ([][]string, error) {
	var list [][]string
	for _, file := range []string{"_list.txt", "___listing___.bin"} {
		f, err := os.Open(filepath.Join(dlcDir, rhgamecd, file))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return nil, err
		}

		defer f.Close()

		cr := csv.NewReader(f)
		cr.Comma = '\t'
		cr.FieldsPerRecord = 6

		list, err = cr.ReadAll()
		if err != nil {
			return nil, err
		}

		break
	}

	// TODO: these have crazy newlines sometimes, fix them?
	for ei, entry := range list {
		list[ei][5] = strings.TrimSpace(entry[5])
	}

	return list, nil
}

func filterDlsList(list [][]string, fields map[string][]byte) [][]string {
	filter := func(value string, index int) {
		dst := list[:0]
		for _, entry := range list {
			if entry[index] == value {
				dst = append(dst, entry)
			}
		}

		list = dst
	}

	attr1, ok := fields["attr1"]
	if ok {
		filter(string(attr1), 2)
	}
	attr2, ok := fields["attr2"]
	if ok {
		filter(string(attr2), 3)
	}
	attr3, ok := fields["attr3"]
	if ok {
		filter(string(attr3), 4)
	}

	return list
}

func dlsContents(moduleName string, fields map[string][]byte) []byte {
	dlcFolder := filepath.Join(dlcDir, string(fields["rhgamecd"]))

	contents, ok := fields["contents"]
	if !ok {
		logging.Error(moduleName, "Missing contents")
		return nil
	}

	file, err := os.ReadFile(filepath.Join(dlcFolder, filepath.Base(string(contents))))
	if err != nil {
		if os.IsNotExist(err) {
			logging.Error(moduleName, "Unknown file:", aurora.Cyan(fields["contents"]))
		}

		return nil
	}

	return file
}

func isValidRHGameCode(rhgamecd string) bool {
	if len(rhgamecd) != 4 {
		return false
	}

	return common.IsUppercaseAlphanumeric(rhgamecd)
}

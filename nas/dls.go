package nas

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"owfc/common"
	"owfc/logging"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

var (
	dlsHandlers = map[string]DownloadHandlerFunc{
		"count":    handleDownload(dlsCount),
		"list":     handleDownload(dlsList),
		"contents": handleDownload(dlsContents),
	}

	dlcDir = "./dlc"
)

type DownloadHandlerFunc func(moduleName string, req url.Values) ([]byte, error)

func handleDownload[reqT any](handler func(string, reqT) ([]byte, error)) DownloadHandlerFunc {
	return func(moduleName string, req url.Values) ([]byte, error) {
		var authToken common.NASAuthToken
		token, _ := common.Base64DwcEncoding.DecodeString(req.Get("token"))
		err := authToken.Unmarshal(string(token))
		if err != nil {
			return nil, err
		}

		in, err := Unmarshal[reqT](req)
		if err != nil {
			return nil, err
		}

		return handler(moduleName, in)
	}
}

func handleDownloadEndpoint(w http.ResponseWriter, r *http.Request) {
	moduleName := "DLS:" + r.RemoteAddr

	action, err := common.Base64DwcEncoding.DecodeString(r.FormValue("action"))
	if err != nil {
		logging.Error(moduleName, "No action in form")
		replyHTTPError(w, 400, "400 Bad Request")
	}

	handler, ok := dlsHandlers[string(action)]
	if !ok {
		logging.Error(moduleName, "Unknown action:", aurora.Cyan(string(action)))
		replyHTTPError(w, 400, "400 Bad Request")
	}

	resp, err := handler(moduleName, r.PostForm)
	if err != nil {
		logging.Error(moduleName, "Action", aurora.Cyan(string(action)), "returned error:", aurora.Cyan(err))

		if os.IsNotExist(err) {
			replyHTTPError(w, 404, "400 Not Found")
			return
		}
	}

	w.Header().Set("X-DLS-Host", "dls1.nintendowifi.net")
	w.Header().Set("Content-Length", strconv.Itoa(len(resp)))
	for chunk := range slices.Chunk([]byte(resp), 1024*4) {
		w.Write(chunk)
	}
}

type DownloadRequest struct {
	GameCode string `nas:"rhgamecd"`
}
type DownloadAttributes struct {
	Attribute1 string `nas:"attr1"`
	Attribute2 string `nas:"attr2"`
	Attribute3 string `nas:"attr3"`
}
type CountRequest struct {
	DownloadRequest
	DownloadAttributes
}

func dlsCount(moduleName string, req CountRequest) ([]byte, error) {
	if !isValidRHGameCode(req.GameCode) {
		return []byte{'0'}, fmt.Errorf("invalid rhgamecd: %s", req.GameCode)
	}

	list, err := getDlsList(req.GameCode)
	if err != nil {
		return []byte{'0'}, fmt.Errorf("unknown game: %s", req.GameCode)
	}

	list = filterDlsList(list, req.Attribute1, req.Attribute2, req.Attribute3)

	return []byte(strconv.Itoa(len(list))), nil
}

type ListRequest struct {
	DownloadRequest
	DownloadAttributes
	Offset int `nas:"offset"`
	Count  int `nas:"num"`
}

func dlsList(moduleName string, req ListRequest) ([]byte, error) {
	if !isValidRHGameCode(req.GameCode) {
		return nil, fmt.Errorf("invalid rhgamecd: %s", req.GameCode)
	}

	list, err := getDlsList(req.GameCode)
	if err != nil {
		return nil, fmt.Errorf("unknown game: %s", req.GameCode)
	}

	list = filterDlsList(list, req.Attribute1, req.Attribute2, req.Attribute3)

	list = list[min(req.Offset, len(list)):]
	if req.Count != 0 {
		list = list[:min(req.Count, len(list))]
	}

	buf := new(bytes.Buffer)
	cw := csv.NewWriter(buf)
	cw.Comma = '\t'
	cw.UseCRLF = true

	err = cw.WriteAll(list)
	if err != nil {
		return nil, err
	}

	buf.WriteString("\r\n")

	return buf.Bytes(), nil
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

func filterDlsList(list [][]string, attr1, attr2, attr3 string) [][]string {
	filter := func(value string, index int) {
		dst := list[:0]
		for _, entry := range list {
			if entry[index] == value {
				dst = append(dst, entry)
			}
		}

		list = dst
	}

	if attr1 != "" {
		filter(attr1, 2)
	}
	if attr2 != "" {
		filter(attr2, 3)
	}
	if attr3 != "" {
		filter(attr3, 4)
	}

	return list
}

type ContentsRequest struct {
	DownloadRequest
	Contents string `nas:"contents"`
}

func dlsContents(moduleName string, req ContentsRequest) ([]byte, error) {
	if !isValidRHGameCode(req.GameCode) {
		return nil, fmt.Errorf("invalid rhgamecd: %s", req.GameCode)
	}

	return os.ReadFile(filepath.Join(dlcDir, req.GameCode, filepath.Base(req.Contents)))
}

func isValidRHGameCode(rhgamecd string) bool {
	if len(rhgamecd) != 4 {
		return false
	}

	return common.IsUppercaseAlphanumeric(rhgamecd)
}

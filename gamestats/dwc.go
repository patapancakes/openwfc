package gamestats

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"owfc/common"
	"owfc/logging"
	"slices"
	"strings"
	"time"
)

type DwcRankingGetHeader struct {
	ProfileID  uint32
	Region     uint32
	Category   uint32
	Mode       uint32
	ModeLength uint32
}

func handleDwcGet(r *http.Request, game common.GameInfo, moduleName string) []byte {
	challenge := makeDwcChallenge(r.PathValue("gamename"), r.PathValue("endpoint"), r.FormValue("pid"))
	if r.FormValue("hash") == "" {
		return []byte(challenge)
	}
	if !verifyDwcHash(game.Stats.Key, challenge, r.FormValue("hash")) {
		logging.Warn(moduleName, "Invalid hash")
		return nil
	}

	data, err := base64.URLEncoding.DecodeString(r.FormValue("data"))
	if err != nil {
		logging.Error(moduleName, "Invalid data:", err)
		return nil
	}

	data = decryptDwc(game.Stats, data)

	reader := bytes.NewReader(data)

	var header DwcRankingGetHeader
	err = binary.Read(reader, binary.LittleEndian, &header)
	if err != nil {
		logging.Error(moduleName, "Invalid header:", err)
		return nil
	}

	var sort, since, limit uint32
	var friends []uint32
	switch header.Mode {
	case common.DwcRankOrder:
		binary.Read(reader, binary.LittleEndian, &sort)
		binary.Read(reader, binary.LittleEndian, &since)
	case common.DwcRankTop, common.DwcRankNear, common.DwcRankNearHigh, common.DwcRankNearLow:
		binary.Read(reader, binary.LittleEndian, &sort)
		binary.Read(reader, binary.LittleEndian, &limit)
		binary.Read(reader, binary.LittleEndian, &since)
	case common.DwcRankFriends:
		binary.Read(reader, binary.LittleEndian, &sort)
		binary.Read(reader, binary.LittleEndian, &limit)
		binary.Read(reader, binary.LittleEndian, &since)
		binary.Read(reader, binary.LittleEndian, &friends)
	default:
		logging.Error(moduleName, "Invalid mode")
		return nil
	}

	// clean up / validate
	desc := sort == 1
	if header.Mode != common.DwcRankOrder && (limit < 1 || limit > 30) {
		logging.Error(moduleName, "Invalid limit")
		return nil
	}
	var sinceTime time.Time
	if since != 0 {
		sinceTime = time.Now().Add(-time.Minute * time.Duration(since))
	}
	friends = slices.DeleteFunc(friends, func(pid uint32) bool { return pid == 0 })

	entries, total, err := db.GetDwcRankings(game.Name, header.ProfileID, int(header.Region), int(header.Category), int(header.Mode), desc, sinceTime, int(limit), friends)
	if err != nil {
		logging.Error(moduleName, "Failed to get rankings:", err)
		return nil
	}

	var respBody bytes.Buffer

	if header.Mode == common.DwcRankOrder {
		binary.Write(&respBody, binary.LittleEndian, uint32(entries[0].Order))
		binary.Write(&respBody, binary.LittleEndian, uint32(total))
	} else {
		binary.Write(&respBody, binary.LittleEndian, uint32(len(entries)))
		binary.Write(&respBody, binary.LittleEndian, uint32(total))

		for _, entry := range entries {
			binary.Write(&respBody, binary.LittleEndian, uint32(entry.Order))
			binary.Write(&respBody, binary.LittleEndian, entry.ProfileID)
			binary.Write(&respBody, binary.LittleEndian, uint32(entry.Score))
			binary.Write(&respBody, binary.LittleEndian, uint32(entry.Region))
			binary.Write(&respBody, binary.LittleEndian, uint32(max(time.Since(entry.Created)/time.Minute, 0)))

			padding := (4 - len(entry.Data)%4) % 4
			paddedSize := len(entry.Data) + padding

			binary.Write(&respBody, binary.LittleEndian, uint32(paddedSize))
			respBody.Write(entry.Data)

			respBody.Write(make([]byte, padding))
		}
	}

	var resp bytes.Buffer

	// write the header
	binary.Write(&resp, binary.LittleEndian, uint32(respBody.Len()))
	binary.Write(&resp, binary.LittleEndian, header.Mode)

	// write the body
	io.Copy(&resp, &respBody)

	// write proof if v2
	if strings.HasSuffix(r.PathValue("endpoint"), "get2.asp") {
		resp.Write([]byte(makeDwcProof(game.Stats.Key, resp.Bytes())))
	}

	return resp.Bytes()
}

type DwcRankingPutHeader struct {
	ProfileID uint32
	Region    uint32
	Category  uint32
	Score     int32
	DataLen   uint32
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

	data, err := base64.URLEncoding.DecodeString(r.FormValue("data"))
	if err != nil {
		logging.Error(moduleName, "Invalid data:", err)
		return nil
	}

	data = decryptDwc(game.Stats, data)

	reader := bytes.NewReader(data)

	var header DwcRankingPutHeader
	err = binary.Read(reader, binary.LittleEndian, &header)
	if err != nil {
		logging.Error(moduleName, "Invalid header:", err)
		return nil
	}

	// HACK: put2 only has an extra length value, just shift them
	if strings.HasSuffix(r.PathValue("endpoint"), "put2.asp") {
		header.Region = header.Category
		header.Category = uint32(header.Score)
		header.Score = int32(header.DataLen)

		err := binary.Read(reader, binary.LittleEndian, &header.DataLen)
		if err != nil {
			logging.Error(moduleName, "Invalid header:", err)
			return nil
		}
	}

	body := make([]byte, header.DataLen)
	_, err = io.ReadAtLeast(reader, body, int(header.DataLen))
	if err != nil {
		logging.Error(moduleName, "Invalid body:", err)
		return nil
	}

	err = db.InsertDwcRanking(game.Name, header.ProfileID, int(header.Region), int(header.Category), int(header.Score), body)
	if err != nil {
		logging.Error(moduleName, "Failed to insert DWC ranking:", err)
		return nil
	}

	response := []byte("done")

	// no captures of how v1 replied, assume it acts like get
	// write proof if v2
	if strings.HasSuffix(r.PathValue("endpoint"), "put2.asp") {
		response = append([]byte(makeDwcProof(game.Stats.Key, response)), response...)
	}

	return response
}

package common

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"sync"

	_ "embed"
)

type GameInfo struct {
	ID          int
	Name        string
	SecretKey   string
	Stats       StatsInfo
	Description string
}

type StatsInfo struct {
	Key          string
	Multiplier   uint32
	Increment    uint32
	Modulus      uint32
	ChecksumMask uint32
}

var (
	gameList           []GameInfo
	readGameList       = false
	gameListIDLookup   = map[int]int{}
	gameListNameLookup = map[string]int{}
	mutex              = sync.RWMutex{}

	//go:embed game_list.tsv
	gameListFile []byte
)

func GetGameInfoByID(gameId int) (GameInfo, bool) {
	ReadGameList()

	mutex.Lock()
	defer mutex.Unlock()

	if index, ok := gameListIDLookup[gameId]; ok && index < len(gameList) {
		return gameList[index], true
	}

	return GameInfo{}, false
}

func GetGameInfoByName(name string) (GameInfo, bool) {
	ReadGameList()

	mutex.Lock()
	defer mutex.Unlock()

	if index, ok := gameListNameLookup[name]; ok && index < len(gameList) {
		return gameList[index], true
	}

	return GameInfo{}, false
}

func GetGameID(name string) int {
	info, ok := GetGameInfoByName(name)
	if !ok {
		return -1
	}

	return info.ID
}

func GetGameIDOrPanic(name string) int {
	id := GetGameID(name)
	if id == -1 {
		panic("Game not found: " + name)
	}

	return id
}

func ReadGameList() {
	mutex.Lock()
	defer mutex.Unlock()

	if readGameList {
		return
	}

	reader := csv.NewReader(bytes.NewReader(gameListFile))
	reader.Comma = '\t'
	csvList, err := reader.ReadAll()
	if err != nil {
		panic(err)
	}

	gameList = []GameInfo{}
	gameListIDLookup = map[int]int{}
	gameListNameLookup = map[string]int{}

	for index, entry := range csvList {
		game := GameInfo{
			ID:        -1,
			Name:      entry[1],
			SecretKey: entry[3],
			Stats: StatsInfo{
				Key: entry[4],
			},
			Description: entry[0],
		}

		if entry[2] != "" {
			game.ID, err = strconv.Atoi(entry[2])
			if err != nil {
				panic(err)
			}
		}

		if entry[5] != "" {
			value, err := strconv.Atoi(entry[5])
			if err != nil {
				panic(err)
			}

			game.Stats.Multiplier = uint32(value)
		}

		if entry[6] != "" {
			value, err := strconv.Atoi(entry[6])
			if err != nil {
				panic(err)
			}

			game.Stats.Increment = uint32(value)
		}

		if entry[7] != "" {
			value, err := strconv.Atoi(entry[7])
			if err != nil {
				panic(err)
			}

			game.Stats.Modulus = uint32(value)
		}

		if entry[8] != "" {
			value, err := strconv.Atoi(entry[8])
			if err != nil {
				panic(err)
			}

			game.Stats.ChecksumMask = uint32(value)
		}

		gameList = append(gameList, game)

		// Create lookup tables
		if game.ID != -1 {
			gameListIDLookup[game.ID] = index
		}
		gameListNameLookup[entry[1]] = index
	}

	readGameList = true
}

func GetExpectedUnitCode(gameName string) byte {
	if strings.HasSuffix(gameName, "wii") || strings.HasSuffix(gameName, "wiiam") {
		return 1
	}

	if gameName == "sneezieswiiw" || gameName == "wormswiiware" || gameName == "wormswiiwaream" {
		return 1
	}

	// Games with weird other regions
	if gameName == "jockracerna" || gameName == "jockracereu" || gameName == "sengo3wiijp" {
		return 1
	}

	// Cross-platform games
	if gameName == "mahjongkcds" || gameName == "puyopuyo7ds" || gameName == "puyopuyo20ds" {
		return 0xff
	}

	return 0
}

package nas

import (
	"bufio"
	"encoding/binary"
	"errors"
	"net/http"
	"os"
	"owfc/common"
	"strings"
	"time"
)

var profanityFilePath = "./profanity.txt"
var profanityFileLines []string = nil
var lastModTime time.Time

var symbolEquivalences = map[rune]rune{
	'1': 'i',
	'0': 'o',
	'5': 's',
	'4': 'a',
	'3': 'e',
	'7': 't',
	'9': 'g',
	'2': 'z',
	'(': 'c',
}

func CacheProfanityFile() error {
	fileInfo, err := os.Stat(profanityFilePath)
	if err != nil {
		return err
	}

	if !fileInfo.ModTime().After(lastModTime) && profanityFileLines != nil {
		return nil
	}

	file, err := os.Open(profanityFilePath)
	if err != nil {
		return err
	}
	defer func() {
		common.ShouldNotError(file.Close())
	}()

	profanityFileLines = nil
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		profanityFileLines = append(profanityFileLines, line)
	}

	if profanityFileLines == nil {
		return errors.New("the file '" + profanityFilePath + "' is empty")
	}

	lastModTime = fileInfo.ModTime()
	return nil
}

func normalizeWord(word string) string {
	var normalized strings.Builder
	for _, char := range word {
		if equivalent, exists := symbolEquivalences[char]; exists {
			normalized.WriteRune(equivalent)
		} else {
			normalized.WriteRune(char)
		}
	}
	return normalized.String()
}

func IsBadWord(word string) (bool, error) {
	if !isProfanityFileCached() {
		err := CacheProfanityFile()
		if err != nil {
			return false, errors.New("the file '" + profanityFilePath + "' has not been cached")
		}

	}

	normalizedWord := normalizeWord(word)
	for _, line := range profanityFileLines {
		if strings.EqualFold(line, normalizedWord) {
			return true, nil
		}
	}

	return false, nil
}

func isProfanityFileCached() bool {
	fileInfo, err := os.Stat(profanityFilePath)
	if err != nil {
		return false
	}
	return profanityFileLines != nil && !fileInfo.ModTime().After(lastModTime)
}

type ProfanityRequest struct {
	NASRequest
	Region   string `nas:"wregion"`
	Encoding string `nas:"wenc"`
	Type     string `nas:"type"`
	Words    string `nas:"words"`
}
type ProfanityResponse struct {
	NASResponse
	Words string `nas:"prwords"`

	WordsA string `nas:"prwordsA"`
	WordsC string `nas:"prwordsC"`
	WordsE string `nas:"prwordsE"`
	WordsJ string `nas:"prwordsJ"`
	WordsK string `nas:"prwordsK"`
	WordsP string `nas:"prwordsP"`
}

func handleAuthProfanityEndpoint(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		replyHTTPError(w, 400, "400 Bad Request")
		return
	}

	req, err := Unmarshal[ProfanityRequest](r.PostForm)
	if err != nil {
		replyHTTPError(w, 400, "400 Bad Request")
		return
	}

	var words string
	switch req.Encoding {
	case "UTF-8":
		words = req.Words
	case "UTF-16LE":
		words = common.UTF16Decode([]byte(req.Words), binary.LittleEndian)
	case "UTF-16BE":
		words = common.UTF16Decode([]byte(req.Words), binary.BigEndian)
	default:
		replyHTTPError(w, 400, "400 Bad Request")
		return
	}

	resp := ProfanityResponse{NASResponse: NASResponse{
		DateTime:   getDateTime(),
		ReturnCode: "000",
	}}

	var prwords strings.Builder
	for word := range strings.SplitSeq(words, "\t") {
		profane, _ := IsBadWord(word)
		if profane {
			prwords.WriteString("1")
			resp.ReturnCode = "040"
			continue
		}

		prwords.WriteString("0")
	}

	resp.Words = prwords.String()

	// Only known value of this field that works this way
	if req.Region == "A" {
		// TODO - The real server seems to handle the input words differently per region? These values are supposed to differ from prwords
		resp.WordsA = resp.Words
		resp.WordsC = resp.Words
		resp.WordsE = resp.Words
		resp.WordsJ = resp.Words
		resp.WordsK = resp.Words
		resp.WordsP = resp.Words
	}

	w.Write([]byte(Marshal(resp)))
}

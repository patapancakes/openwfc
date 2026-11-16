package qr2

import (
	"encoding/base64"
	"fmt"
	"net"
	"owfc/common"
	"owfc/logging"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

func (s *Session) handleChallenge(moduleName string, conn net.PacketConn, buffer []byte) {
	game, ok := common.GetGameInfoByName(s.Data["gamename"])
	if !ok {
		logging.Error(moduleName, "Unknown game:", aurora.Cyan(s.Data["gamename"]))
		return
	}

	challenge, err := common.GetString(buffer)
	if err != nil {
		logging.Error(moduleName, "Invalid challenge")
		return
	}

	if challenge != encode(encrypt([]byte(game.SecretKey), []byte(s.Challenge))) {
		logging.Error(moduleName, "Invalid challenge")
		return
	}

	s.Authenticated = true

	var data strings.Builder
	for k, v := range s.Data {
		if k == "gamename" || k == "statechanged" {
			continue
		}

		if data.Len() != 0 {
			data.WriteString(", ")
		}

		fmt.Fprintf(&data, "%s: %s", k, v)
	}

	logging.Notice(moduleName, "Server created:", aurora.Cyan(game.Name), "/", aurora.Cyan(&data))
	logging.Event("server_created", map[string]any{
		"cookie":     s.Cookie,
		"data":       s.Data,
		"ip_address": s.Addr.String(),
	})

	conn.WriteTo(createResponseHeader(ClientRegisteredReply, s.Cookie), s.Addr)
}

func encode(data []byte) string {
	padded := make([]byte, ((len(data)+2)/3)*3)
	copy(padded, data)

	return base64.StdEncoding.EncodeToString(padded)
}

func encrypt(key []byte, data []byte) []byte {
	var state [256]byte

	swap := func(a *byte, b *byte) {
		swap := *a
		*a = *b
		*b = swap
	}

	for counter := range state {
		state[counter] = byte(counter)
	}

	var x, y uint8
	for counter := range state {
		y = key[x] + state[counter] + y
		x = (x + 1) % uint8(len(key))
		swap(&state[counter], &state[y])
	}

	x = 0
	y = 0

	var xorIdx uint8
	for counter := range data {
		x = x + data[counter] + 1
		y = state[x] + y
		swap(&state[x], &state[y])
		xorIdx = state[x] + state[y]
		data[counter] ^= state[xorIdx]
	}

	return data
}

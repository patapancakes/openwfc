package common

// TODO: should probably be in gpcm, qr2 needs it though

const (
	GPStatusOffline = iota
	GPStatusOnline
	GPStatusPlaying
	GPStatusMatchAnybody
	GPStatusMatchFriend

	StatusMatchClient
	StatusMatchServer
)

func GetStatusString(status int) string {
	switch status {
	case GPStatusOffline:
		return "OFFLINE"
	case GPStatusOnline:
		return "ONLINE"
	case GPStatusPlaying:
		return "PLAYING"
	case GPStatusMatchAnybody:
		return "MATCH_ANYBODY"
	case GPStatusMatchFriend:
		return "MATCH_FRIEND"
	case StatusMatchClient:
		return "MATCH_SC_CL"
	case StatusMatchServer:
		return "MATCH_SC_SV"
	}

	return "UNKNOWN"
}

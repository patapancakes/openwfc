package gpcm

const (
	StatusOffline = iota
	StatusOnline
	StatusPlaying
	StatusMatchAnybody
	StatusMatchFriend

	StatusMatchClient
	StatusMatchServer
)

func GetStatusString(status int) string {
	switch status {
	case StatusOffline:
		return "OFFLINE"
	case StatusOnline:
		return "ONLINE"
	case StatusPlaying:
		return "PLAYING"
	case StatusMatchAnybody:
		return "MATCH_ANYBODY"
	case StatusMatchFriend:
		return "MATCH_FRIEND"
	case StatusMatchClient:
		return "MATCH_SC_CL"
	case StatusMatchServer:
		return "MATCH_SC_SV"
	}

	return "UNKNOWN"
}

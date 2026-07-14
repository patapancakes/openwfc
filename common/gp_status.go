package common

// GP statuses
// should probably be in gpcm, qr2 needs it though

const (
	Offline = iota
	Online
	Playing
	MatchAnybody
	MatchFriend

	MatchClient
	MatchServer
)

func GetStatusString(status int) string {
	switch status {
	case Offline:
		return "OFFLINE"
	case Online:
		return "ONLINE"
	case Playing:
		return "PLAYING"
	case MatchAnybody:
		return "MATCH_ANYBODY"
	case MatchFriend:
		return "MATCH_FRIEND"
	case MatchClient:
		return "MATCH_SC_CL"
	case MatchServer:
		return "MATCH_SC_SV"
	}

	return "UNKNOWN"
}

package common

const (
	MatchTypeAnybody = iota
	MatchTypeFriend
	MatchTypeServer
	MatchTypeClient
)

func GetMatchTypeString(matchType int) string {
	switch matchType {
	case MatchTypeAnybody:
		return "MATCH_ANYBODY"
	case MatchTypeFriend:
		return "MATCH_FRIEND"
	case MatchTypeServer:
		return "MATCH_SC_SV"
	case MatchTypeClient:
		return "MATCH_SC_CL"
	}

	return "UNKNOWN"
}

package common

const (
	MatchStateInit = iota

	MatchStateClientWaiting
	MatchStateClientSearchOwn
	MatchStateClientSearchHost
	MatchStateClientWaitReserve
	MatchStateClientSearchNatNegHost
	MatchStateClientNatNeg
	MatchStateClientGT2
	MatchStateClientCancelSyn
	MatchStateClientSyn

	MatchStateServerWaiting
	MatchStateServerOwnNatNeg
	MatchStateServerOwnGT2
	MatchStateServerWaitClientLink
	MatchStateServerCancelSyn
	MatchStateServerCancelSynWait
	MatchStateServerSyn
	MatchStateServerSynWait

	MatchStateWaitClose

	MatchStateServerPollTimeout
)

func GetMatchStateString(state int) string {
	switch state {
	case MatchStateInit:
		return "MATCH_INIT"

	case MatchStateClientWaiting:
		return "MATCH_CL_WAITING"
	case MatchStateClientSearchOwn:
		return "MATCH_CL_SEARCH_OWN"
	case MatchStateClientSearchHost:
		return "MATCH_CL_SEARCH_HOST"
	case MatchStateClientWaitReserve:
		return "MATCH_CL_WAIT_RESV"
	case MatchStateClientSearchNatNegHost:
		return "MATCH_CL_SEARCH_NN_HOST"
	case MatchStateClientNatNeg:
		return "MATCH_CL_NN"
	case MatchStateClientGT2:
		return "MATCH_CL_GT2"
	case MatchStateClientCancelSyn:
		return "MATCH_CL_CANCEL_SYN"
	case MatchStateClientSyn:
		return "MATCH_CL_SYN"

	case MatchStateServerWaiting:
		return "MATCH_SV_WAITING"
	case MatchStateServerOwnNatNeg:
		return "MATCH_SV_OWN_NN"
	case MatchStateServerOwnGT2:
		return "MATCH_SV_OWN_GT2"
	case MatchStateServerWaitClientLink:
		return "MATCH_SV_WAIT_CL_LINK"
	case MatchStateServerCancelSyn:
		return "MATCH_SV_CANCEL_SYN"
	case MatchStateServerCancelSynWait:
		return "MATCH_SV_CANCEL_SYN_WAIT"
	case MatchStateServerSyn:
		return "MATCH_SV_SYN"
	case MatchStateServerSynWait:
		return "MATCH_SV_SYN_WAIT"

	case MatchStateWaitClose:
		return "MATCH_WAIT_CLOSE"

	case MatchStateServerPollTimeout:
		return "MATCH_SV_POLL_TIMEOUT"
	}

	return "UNKNOWN"
}

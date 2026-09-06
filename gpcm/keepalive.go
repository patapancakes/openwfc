package gpcm

type KARequestResponse struct {
	Command string `gs:"ka"`
}

func KeepAlive(state *GameSpySession, req KARequestResponse) (KARequestResponse, error) {
	return KARequestResponse{}, nil
}

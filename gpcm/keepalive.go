package gpcm

type KARequestResponse struct {
	Command string `gs:"ka"`
}

func keepAlive(state *GameSpySession, req KARequestResponse) (KARequestResponse, error) {
	return KARequestResponse{}, nil
}

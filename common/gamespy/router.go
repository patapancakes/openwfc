package gamespy

type HandlerFunc func(state any, req string) (string, error)

type NoResponse struct{}

func Handle[stateT, reqT, respT any](handler func(stateT, reqT) (respT, error)) HandlerFunc {
	return func(state any, req string) (string, error) {
		in, err := Unmarshal[reqT](req)
		if err != nil {
			return "", err
		}

		resp, err := handler(state.(stateT), in)
		if err != nil {
			return "", err
		}

		return Marshal(resp), nil
	}
}

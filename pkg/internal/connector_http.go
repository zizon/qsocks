package internal

type httpConnectorBundle struct {
	ctx     CanclableContext
	gateway string
	key     []byte
}

func httpConnector(bundle httpConnectorBundle) (raceConnector, error) {
	//client := http.Client{}
	return func(connBundle connectBundle) {
		/*
			req, err := http.NewRequest("OPTION", bundle.gateway, nil)
			if err != nil {
				bundle.ctx.CollectError(err)
				return
			}
		*/
	}, nil
}

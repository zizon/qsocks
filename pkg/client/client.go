package client

import (
	"context"
	"time"
)

type Config struct {
	context.Context
	context.CancelFunc
	Listen           string
	Connect          []string
	Timeout          time.Duration
	StreamPerSession int
	Async            bool
}

func Run(config Config) error {
	streamCh := preflight{
		Context:    config.Context,
		target:     config.Connect,
		timeout:    config.Timeout,
		maxStreams: config.StreamPerSession,
		async:      config.Async,
	}.create()

	localCh := local{
		Context: config.Context,
		cancle:  config.CancelFunc,
		listen:  config.Listen,
	}.create()

	go func() {
		for c := range localCh {
			go tunnel{
				from: c,
				toCh: streamCh,
			}.serve()
		}
	}()

	<-config.Done()
	return config.Err()
}

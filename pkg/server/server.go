package server

import (
	"context"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/zizon/qsocks/pkg/logging"
)

type Config struct {
	context.Context
	context.CancelFunc
	Listen string
}

func Run(config Config) error {
	defer config.CancelFunc()

	// create listener
	l, err := quic.ListenAddrEarly(config.Listen, generateTLSConfig(), &quic.Config{
		EnableDatagrams: true,
		KeepAlivePeriod: 5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("fail to create quic listener:%v", err)
	}
	defer l.Close()

	logging.Info("listening %v", l.Addr())
	for {
		c, err := l.Accept(config.Context)
		if err != nil {
			return fmt.Errorf("fail accept quic connection:%v", err)
		}

		go connection{
			Context:    config.Context,
			Connection: c,
		}.serve()
	}
}

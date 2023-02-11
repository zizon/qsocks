package server

import (
	"context"
	"fmt"

	"github.com/quic-go/quic-go"
	"github.com/zizon/qsocks/pkg/logging"
)

type connection struct {
	context.Context
	quic.Connection
}

func (c connection) serve() {
	for {
		s, err := c.AcceptStream(c.Context)
		if err != nil {
			c.close(fmt.Sprintf("connection:%v fail to accept stream:%v", c, err))
			return
		}

		go tunnel{
			Stream: s,
		}.serve()
	}
}

func (c connection) close(reason string) {
	logging.Info("close conneciton:%v reason:%v", c, reason)
	c.CloseWithError(0, reason)
}

func (c connection) String() string {
	return fmt.Sprintf("(%s)", c.RemoteAddr())
}

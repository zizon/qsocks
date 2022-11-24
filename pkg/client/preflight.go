package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/zizon/qsocks/pkg/logging"
	"github.com/zizon/qsocks/pkg/protocol"
)

type preflight struct {
	context.Context
	target     []string
	timeout    time.Duration
	maxStreams int
}

type quicConnection struct {
	context.Context
	quic.EarlyConnection
	maxStreams int
	ready      chan<- quic.Stream
}

func (p preflight) create() <-chan quic.Stream {
	ch := make(chan quic.Stream)

	go func() {
		defer close(ch)

		selected := 0
		for {
			// in case of termination
			select {
			case <-p.Done():
				logging.Info("stop preflight:%v", p.Err())
				return
			default:
			}

			// create session
			c, err := quic.DialAddrEarly(p.target[selected%len(p.target)],
				&tls.Config{
					InsecureSkipVerify: true,
					NextProtos:         protocol.PeerQuicProtocol,
				},
				&quic.Config{
					HandshakeIdleTimeout: p.timeout,
					EnableDatagrams:      true,
					KeepAlivePeriod:      p.timeout,
				},
			)
			if err != nil {
				logging.Warn("fail to create quic session to%v reason:%v", p.target[selected%len(p.target)], err)
				selected = (selected + 1) % len(p.target)
				continue
			}

			// serve connection
			quicConnection{
				Context:         p.Context,
				EarlyConnection: c,
				maxStreams:      p.maxStreams,
				ready:           ch,
			}.serve()
		}
	}()

	return ch
}

func (c quicConnection) serve() {
	cleanup := make([]context.Context, c.maxStreams)
	logging.Info("preflight quic connection:%v", c)
	for i := 0; i < c.maxStreams; i++ {
		s, err := c.OpenStream()
		if err != nil {
			c.close(fmt.Sprintf("fail to create stream of connection:%v, terminate reason:%v", c, err))
			// close connection will therefor close streams,
			// no cleanup needed
			return
		}

		// mark cleanup
		cleanup[i] = s.Context()

		// then push ready one
		select {
		case c.ready <- s:
		case <-c.Done():
			// parent terminated
			c.close(fmt.Sprintf("destroy quic connection:%v as parent die:%v", c, err))
			return
		}
	}

	// then for cleanup
	go func() {
		for _, ctx := range cleanup {
			if ctx != nil {
				<-ctx.Done()
			}
		}

		c.close(fmt.Sprintf("quic managed stream are closed, cleanup connection:%v", c))
	}()
}

func (c quicConnection) close(reason string) {
	logging.Info(reason)
	c.CloseWithError(0, reason)
}

func (c quicConnection) String() string {
	return fmt.Sprintf("(%s->%s)", c.LocalAddr(), c.RemoteAddr())
}

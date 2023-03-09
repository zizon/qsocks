package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/zizon/qsocks/pkg/logging"
	"github.com/zizon/qsocks/pkg/protocol"
)

type preflight struct {
	context.Context
	target     []string
	timeout    time.Duration
	maxStreams int
	async      bool
}

type quicConnection struct {
	quic.Connection
	maxStreams int
	ready      chan<- quic.Stream
}

func (p preflight) create() <-chan quic.Stream {
	ch := make(chan quic.Stream)

	wg := sync.WaitGroup{}
	for _, host := range p.target {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			p.generate(host, ch)
		}(host)
	}

	go func() {
		defer close(ch)
		wg.Wait()
	}()

	return ch
}

func (p preflight) generate(host string, ch chan<- quic.Stream) {
	for {
		// in case of termination
		select {
		case <-p.Done():
			logging.Info("stop preflight:%v", p.Err())
			return
		default:
		}

		// create session
		c, err := func() (quic.Connection, error) {
			if p.async {
				return quic.DialAddrEarlyContext(
					p,
					host,
					&tls.Config{
						InsecureSkipVerify: true,
						NextProtos:         protocol.PeerQuicProtocol,
					},
					&quic.Config{
						HandshakeIdleTimeout: p.timeout,
						EnableDatagrams:      true,
						KeepAlivePeriod:      p.timeout,
					})
			}
			return quic.DialAddrContext(
				p,
				host,
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
		}()
		if err != nil {
			logging.Warn("fail to create quic session to %v reason:%v", host, err)
			continue
		}

		// serve connection
		quicConnection{
			Connection: c,
			maxStreams: p.maxStreams,
			ready:      ch,
		}.serve()
	}
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
		case <-c.Context().Done():
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

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/zizon/qsocks/pkg/logging"
	"github.com/zizon/qsocks/pkg/protocol"
	"github.com/zizon/qsocks/pkg/stream"
)

type Config struct {
	context.Context
	Listen           string
	Connect          string
	Timeout          time.Duration
	StreamPerSession int
}

type Client interface {
	context.Context
}

type client struct {
	context.Context
	socket  stream.State[net.Conn]
	qsocket stream.State[sessionStream]
}

type sessionStream struct {
	quic.Connection
	quic.Stream
	*sync.WaitGroup
}

func NewClient(connect Config) (Client, error) {
	c := &client{
		Context: connect,
	}

	if err := c.setupSocketSteram(connect); err != nil {
		return nil, fmt.Errorf("fail to create socket streams:%v", err)
	}

	c.setupQuic(connect)

	stream.Drain(stream.Reduce(c.qsocket, c.socket, func(q sessionStream, c net.Conn) (any, error) {
		// local speculate
		go func() {
			defer c.Close()
			defer q.Close()
			defer q.Done()

			logging.Info("piping %v -> %v", c.RemoteAddr(), q.remote())

			// 1. auth reply
			if err := (protocol.AuthReply{}).Encode(c); err != nil {
				logging.Error("fail auth reply for stream:%v reaosn:%v", c.RemoteAddr(), err)
				return
			}

			// 2. local forwrad reply
			addr, ok := c.LocalAddr().(*net.TCPAddr)
			if !ok {
				logging.Error("expect addr to be tcp addr:%v from:%v", c.LocalAddr(), c.RemoteAddr())
				return
			} else if err := (&protocol.Reply{
				HOST: addr.IP,
				PORT: addr.Port,
			}).Encode(c); err != nil {
				logging.Error("fail to encode reply -> %v reason:%v", c.RemoteAddr(), err)
				return
			}

			// 3. copy
			if _, err := io.Copy(c, q); err != nil {
				logging.Warn("piping %v -> %v fail:%v", q.remote(), c.RemoteAddr(), err)
			}
		}()

		// quic speculate
		go func() {
			defer c.Close()
			defer q.Close()

			if err := (&protocol.Auth{}).Decode(c); err != nil {
				logging.Error("unkonw auth for stream:%v reason:%v", c.RemoteAddr(), err)
				return
			}

			// forward reqeust and the maybe proxy content
			if _, err := io.Copy(q, c); err != nil {
				logging.Warn("piping %v -> %v fail:%v", c.RemoteAddr(), q.remote(), err)
			}
		}()

		return nil, nil
	}), nil)
	return c, nil
}

func (c *client) setupSocketSteram(connect Config) error {
	l, err := net.Listen("tcp", connect.Listen)
	if err != nil {
		return fmt.Errorf("fail to listen on:%v reason:%v", connect.Listen, err)
	}
	logging.Info("listening %v", l.Addr())

	// socket listner
	c.socket = stream.Of(func() (net.Conn, error) {
		conn, err := l.Accept()
		if err != nil {
			l.Close()
			return nil, fmt.Errorf("fail accept from:%v reason:%v", l.Addr(), err)
		}
		return conn, nil
	})

	return nil
}

func (c *client) setupQuic(connect Config) error {
	sessions := stream.Of(func() (quic.Connection, error) {
		for {
			session, err := quic.DialAddrEarly(connect.Connect,
				&tls.Config{
					InsecureSkipVerify: true,
					NextProtos:         protocol.PeerQuicProtocol,
				},
				&quic.Config{
					HandshakeIdleTimeout: connect.Timeout,
					MaxIdleTimeout:       connect.Timeout,
					EnableDatagrams:      true,
					KeepAlivePeriod:      connect.Timeout,
				},
			)
			if err != nil {
				logging.Warn("fail to create quic session:%v", err)
				continue
			}

			logging.Info("preflight quic session:%v->%v", session.LocalAddr(), session.RemoteAddr())
			return session, nil
		}
	})

	c.qsocket = stream.Flatten(sessions, func(session quic.Connection) (stream.State[sessionStream], error) {
		ch := make(chan sessionStream)
		go func() {
			wg := &sync.WaitGroup{}
			defer func() {
				close(ch)
				wg.Wait()
				session.CloseWithError(0, "cleanup")
				logging.Info("retire session:%v limit:%v", session.LocalAddr(), connect.StreamPerSession)
			}()

			for i := 0; i < connect.StreamPerSession; i++ {
				s, err := session.OpenStream()
				if err != nil {
					logging.Error("fail to create session stream:%v, reason:%v", session.LocalAddr(), err)
					return
				}

				wg.Add(1)
				ch <- sessionStream{
					Connection: session,
					Stream:     s,
					WaitGroup:  wg,
				}
			}
		}()

		return stream.From(ch), nil
	}, true)

	return nil
}

func (s *sessionStream) remote() string {
	return fmt.Sprintf("%v[%v](%v)", s.RemoteAddr(), s.StreamID(), s.LocalAddr())
}

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
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
	qsocket stream.State[quic.Stream]
}

func NewClient(connect Config) (Client, error) {
	c := &client{
		Context: connect,
	}

	if err := c.setupSocketSteram(connect); err != nil {
		return nil, fmt.Errorf("fail to create socket streams:%v", err)
	}

	c.setupQuic(connect)

	stream.Drain(stream.Reduce(c.qsocket, c.socket, func(q quic.Stream, c net.Conn) (any, error) {
		// local speculate
		go func() {
			defer c.Close()

			logging.Info("conencting %v -> %v", c.RemoteAddr(), q.StreamID())

			// 1. auth reply
			if err := (protocol.AuthReply{}).Encode(c); err != nil {
				logging.Error("fail auth reply for stream reaosn:%v", err)
				return
			}

			// 2. local forwrad reply
			addr, ok := c.LocalAddr().(*net.TCPAddr)
			if !ok {
				logging.Error("expect addr to be tcp addr:%v", c.LocalAddr())
				return
			} else if err := (&protocol.Reply{
				HOST: addr.IP,
				PORT: addr.Port,
			}).Encode(c); err != nil {
				logging.Error("fail to encode reply reason:%v", err)
				return
			}

			// 3. copy
			if _, err := io.Copy(c, q); err != nil {
				logging.Warn("fail piping %v -> %v, reason:%v", q.StreamID(), c.RemoteAddr(), err)
			}
		}()

		// quic speculate
		go func() {
			if err := (&protocol.Auth{}).Decode(c); err != nil {
				logging.Error("unkonw auth for stream:%v reaosn:%v", q.StreamID(), err)
				return
			}

			// forward reqeust and the maybe proxy content
			if _, err := io.Copy(q, c); err != nil {
				logging.Warn("fail piping %v -> %v, reason:%v", c.RemoteAddr(), q.StreamID(), err)
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
			return nil, fmt.Errorf("fail accept from:%v reason:%v", l.Addr(), err)
		}
		return conn, nil
	})

	return nil
}

func (c *client) setupQuic(connect Config) error {
	sessions := stream.Of(func() (quic.Session, error) {
		for {
			session, err := quic.DialAddrEarly(connect.Connect,
				&tls.Config{
					InsecureSkipVerify: true,
					NextProtos:         protocol.PeerQuicProtocol,
				},
				&quic.Config{
					HandshakeIdleTimeout: connect.Timeout,
					MaxIdleTimeout:       connect.Timeout,
					KeepAlive:            true,
					EnableDatagrams:      true,
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

	c.qsocket = stream.Flatten(sessions, func(session quic.Session) (stream.State[quic.Stream], error) {
		limit := connect.StreamPerSession
		return stream.Of(func() (quic.Stream, error) {
			if limit > 0 {
				limit -= 0
				return session.OpenStream()
			}

			return nil, io.EOF
		}), nil
	}, true)

	return nil
}

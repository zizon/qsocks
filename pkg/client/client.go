package client

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/zizon/qsocks/pkg/logging"
	"github.com/zizon/qsocks/pkg/protocol"
)

type Config struct {
	context.Context
	context.CancelFunc
	Listen           string
	Connect          string
	Timeout          time.Duration
	StreamPerSession int
}

type Client interface {
	context.Context
}

func NewClient(connect Config) (Client, error) {
	streamCh := make(chan quic.Stream)
	// preflight
	go func() {
		for {
			select {
			case <-connect.Done():
				return
			default:
			}

			session, err := quic.DialAddrEarly(connect.Connect,
				&tls.Config{
					InsecureSkipVerify: true,
					NextProtos:         protocol.PeerQuicProtocol,
				},
				&quic.Config{
					HandshakeIdleTimeout: connect.Timeout,
					EnableDatagrams:      true,
					KeepAlivePeriod:      connect.Timeout,
				},
			)
			if err != nil {
				logging.Warn("fail to create quic session:%v", err)
				continue
			}

			logging.Info("preflight quic session:%v->%v", session.LocalAddr(), session.RemoteAddr())
			go func() {
				defer session.CloseWithError(0, "close session")

				for i := 0; i < connect.StreamPerSession; i++ {
					s, err := session.OpenStream()
					if err != nil {
						logging.Warn("fail to open stream of session:%v->%v reason:%v",
							session.LocalAddr(), session.RemoteAddr(), err)
						return
					}

					streamCh <- s
				}
			}()
		}
	}()

	// local listening
	go func() {
		defer connect.CancelFunc()

		l, err := net.Listen("tcp", connect.Listen)
		if err != nil {
			logging.Error("fail to listen on:%v reason:%v", connect.Listen, err)
			return
		}
		defer l.Close()

		logging.Info("listening %v", l.Addr())
		for {
			c, err := l.Accept()
			if err != nil {
				logging.Error("fail to accept local connection:%v", err)
				return
			}

			select {
			case <-connect.Done():
				c.Close()
				return
			default:
				// server one conenction
				go func() {
					defer c.Close()

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

					select {
					case <-connect.Done():
					case s := <-streamCh:
						defer s.Close()

						logging.Info("piping %v -> %v", c.RemoteAddr(), s.StreamID())

						// 3. copy from remote
						if _, err := io.Copy(c, s); err != nil {
							logging.Warn("piping %v -> %v fail:%v", s.StreamID(), c.RemoteAddr(), err)
							return
						}

						// 4. copy to remote
						go func() {
							// consume the auth request
							if err := (&protocol.Auth{}).Decode(c); err != nil {
								logging.Error("unkonw auth for stream:%v reason:%v", c.RemoteAddr(), err)
								return
							}

							// since the forward message were not decode, simply forward to remote
							if _, err := io.Copy(s, c); err != nil {
								logging.Warn("piping %v -> %v fail:%v", c.RemoteAddr(), s.StreamID(), err)
								return
							}
						}()
					}
				}()
			}
		}
	}()

	return connect, nil
}

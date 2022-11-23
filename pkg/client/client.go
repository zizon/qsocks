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
	streamCh := preflight(connect)
	localCh := acceptLocal(connect)

	go func() {
		for c := range localCh {
			go serve(c, streamCh)
		}
	}()

	return connect, nil
}

func preflight(connect Config) <-chan quic.Stream {
	ch := make(chan quic.Stream)
	go func() {
		defer close(ch)

		for {
			select {
			case <-connect.Done():
				logging.Info("stop preflight:%v", connect.Err())
				return
			default:
			}

			// create session
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

			func() {
				join := &struct {
					ctx []context.Context
				}{
					ctx: []context.Context{},
				}
				defer func() {
					for _, ctx := range join.ctx {
						<-ctx.Done()
					}

					session.CloseWithError(0, "use up stream quota per session")
				}()
				logging.Info("preflight quic session:%v->%v", session.LocalAddr(), session.RemoteAddr())

				for i := 0; i < connect.StreamPerSession; i++ {
					s, err := session.OpenStream()
					if err != nil {
						logging.Error("fail to open stream of session:%v->%v reason:%v",
							session.LocalAddr(), session.RemoteAddr(), err)
						return
					} else {
						join.ctx = append(join.ctx, s.Context())
						select {
						case ch <- s:
						case <-connect.Done():
							logging.Info("terminate session:%v", connect.Err())
							// session close will cause stream close, so no need to do closed seperately
							return
						}
					}
				}
			}()
		}
	}()
	return ch
}

func acceptLocal(connect Config) <-chan net.Conn {
	ch := make(chan net.Conn)

	go func() {
		defer close(ch)
		l, err := net.Listen("tcp", connect.Listen)
		if err != nil {
			logging.Error("fail to listen on:%v reason:%v", connect.Listen, err)
			return
		}
		defer l.Close()

		for {
			c, err := l.Accept()
			if err != nil {
				logging.Error("fail to accept local connection:%v", err)
				return
			}

			select {
			case <-connect.Done():
				logging.Info("terminate local accept:%v", connect.Err())
				c.Close()
				return
			case ch <- c:
			}
		}
	}()

	return ch
}

func serve(c net.Conn, ch <-chan quic.Stream) {
	defer c.Close()

	// 1. consume the auth request
	if err := (&protocol.Auth{}).Decode(c); err != nil {
		logging.Error("unkonw auth for stream:%v reason:%v", c.RemoteAddr(), err)
		return
	}

	// 2. poll a Stream
	s, more := <-ch
	if !more {
		logging.Info("no avaliable stream, stop serving:%v", c.RemoteAddr())
		return
	}

	// 3. pipe to remote
	go func() {
		defer s.Close()
		// since the forward message were not decode, simply forward to remote
		if _, err := io.Copy(s, c); err != nil {
			logging.Error("fail to pipe local:%v->%v reason:%v", c.RemoteAddr(), s.StreamID(), err)
		}
	}()

	// 4. auth reply
	if err := (protocol.AuthReply{}).Encode(c); err != nil {
		logging.Error("fail auth reply for stream:%v reaosn:%v", c.RemoteAddr(), err)
		return
	}

	// 5. local forwrad reply
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

	// 6. pipe from remote
	if _, err := io.Copy(c, s); err != nil {
		logging.Error("fail to pipe remote:%v->%v reason:%v", s, c.RemoteAddr(), err)
	}
}

package internal

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	quic "github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/logging"
	"github.com/lucas-clemente/quic-go/qlog"
)

type localQuicServer struct {
	CanclableContext
}

type upstream struct {
	CanclableContext
	quic.Stream
}

type tracer struct {
	logging.Perspective
	connectionID []byte
}

func (t tracer) Write(b []byte) (int, error) {
	LogTrace("p:%s connectionID:%v trace:%s", t.Perspective, t.connectionID, b)
	return len(b), nil
}

func (t tracer) Close() error {
	return nil
}

func StartSessionLimitedSocks5RaceServer(ctx context.Context, listen, connect string, timeout time.Duration, streamPerSession int) CanclableContext {
	s := &localQuicServer{
		CanclableContext: NewCanclableContext(ctx),
	}

	go func() {
		l, err := net.Listen("tcp", listen)
		if err != nil {
			s.Cancle(fmt.Errorf("fail to listen at:%v reason:%v", listen, err))
			return
		}

		streams := make(chan *upstream)
		go func() {
			// generate stream
			for {
				select {
				case <-s.Done():
					return
				default:
					session, err := quic.DialAddrEarly(connect,
						&tls.Config{
							InsecureSkipVerify: true,
							NextProtos:         peerQuicProtocol,
						},
						&quic.Config{
							Tracer: qlog.NewTracer(func(p logging.Perspective, connectionID []byte) io.WriteCloser {
								return tracer{
									Perspective:  p,
									connectionID: connectionID,
								}
							}),
							HandshakeIdleTimeout: timeout,
							MaxIdleTimeout:       timeout,
							KeepAlive:            true,
							EnableDatagrams:      true,
						},
					)
					if err != nil {
						LogWarn("fail to setup upstream:%v reason:%v", connect, err)
						continue
					}

					sessionCtx := s.Fork()
					sessionCtx.OnCancle(func(err error) {
						session.CloseWithError(0, fmt.Sprintf("session close:%v", err))
					})

					active := &sync.WaitGroup{}
					for i := 0; i < streamPerSession; i++ {
						stream, err := session.OpenStream()
						if err != nil {
							sessionCtx.Cancle(fmt.Errorf("session fail to open stream:%v reason:%v", session, err))
							break
						}

						active.Add(1)
						streamCtx := sessionCtx.Fork()
						streamCtx.OnCancle(func(err error) {
							stream.Close()
							active.Done()
						})

						// ready
						streams <- &upstream{
							CanclableContext: streamCtx,
							Stream:           stream,
						}
					}

					go func() {
						// wait of session stream die
						active.Wait()
						sessionCtx.Cancle(nil)
					}()
				}
			}
		}()

		for {
			c, err := l.Accept()
			LogInfo("accept one:%v", c.RemoteAddr())
			if err != nil {
				s.Cancle(fmt.Errorf("fail to accept for:%v reason:%v", listen, err))
				return
			}

			// poll one
			stream := <-streams

			ctx := s.Fork()
			ctx.OnCancle(func(err error) {
				c.Close()
				stream.Cancle(fmt.Errorf("local connection close:%v reason:%v", c, err))
			})

			// forward upstream
			go func() {
				if _, err := io.Copy(stream, c); err != nil {
					ctx.Cancle(fmt.Errorf("fail to forward local::%v to remote:%v reason:%v", c, stream, err))
					return
				}

				ctx.Cancle(nil)
			}()

			// for remote reply
			go func() {
				// 1. read auth reply
				authReply := AuthReply{}
				if err := authReply.Decode(stream); err != nil {
					ctx.Cancle(fmt.Errorf("fail read auth reply for stream reaosn:%v", err))
					return
				}

				// 2. forward auth reply to client
				if err := authReply.Encode(c); err != nil {
					ctx.Cancle(fmt.Errorf("fail to reply to client:%v", err))
					return
				}

				// 3. local reply, since remove reply is useless
				addr, ok := l.Addr().(*net.TCPAddr)
				if !ok {
					ctx.Cancle(fmt.Errorf("expect addr to be tcp addr:%v", l))
					return
				}

				if err := (&Reply{
					HOST: addr.IP,
					PORT: addr.Port,
				}).Encode(c); err != nil {
					ctx.Cancle(fmt.Errorf("fail to encode reply:%v reason:%v", c, err))
					return
				}

				// 5. start Copy
				if _, err := io.Copy(c, stream); err != nil {
					ctx.Cancle(fmt.Errorf("fail to write to local:%v, reason:%v", c, err))
					return
				}

				// normal end
				ctx.Cancle(nil)
			}()
		}
	}()

	return s
}

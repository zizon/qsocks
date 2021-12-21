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
		s.OnCancle(func(err error) {
			LogInfo("close listener:%v reason:%v", listen, err)
			l.Close()
		})

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
						msg := fmt.Sprintf("session close:%v", err)
						LogInfo(msg)
						session.CloseWithError(0, msg)
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
							LogInfo("stream closed reason:%v", err)
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
			if err != nil {
				s.Cancle(fmt.Errorf("fail to accept for:%v reason:%v", listen, err))
				return
			}
			LogInfo("accept one:%v", c.RemoteAddr())

			// poll one
			stream := <-streams

			ctx := s.Fork()
			ctx.OnCancle(func(err error) {
				c.Close()
				stream.Cancle(fmt.Errorf("local connection close reason:%v", err))
			})

			// speculate reply
			go func() {
				// 1. auth reply
				if err := (AuthReply{}).Encode(c); err != nil {
					ctx.Cancle(fmt.Errorf("fail auth reply for stream reaosn:%v", err))
					return
				}

				// 2. local forwrad reply
				addr, ok := l.Addr().(*net.TCPAddr)
				if !ok {
					ctx.Cancle(fmt.Errorf("expect addr to be tcp addr:%v", l))
					return
				} else if err := (&Reply{
					HOST: addr.IP,
					PORT: addr.Port,
				}).Encode(c); err != nil {
					ctx.Cancle(fmt.Errorf("fail to encode reply reason:%v", err))
					return
				}

				// 3. copy
				if _, err := io.Copy(c, stream); err != nil {
					ctx.Cancle(fmt.Errorf("fail to write to local:%v, reason:%v", c, err))
				} else {
					ctx.Cancle(nil)
				}
			}()

			// drop auth request
			go func() {
				if err := (&Auth{}).Decode(c); err != nil {
					ctx.Cancle(fmt.Errorf("unkonw auth for stream:%v reaosn:%v", s, err))
					return
				}

				// forward reqeust and the maybe proxy content
				if _, err := io.Copy(stream, c); err != nil {
					ctx.Cancle(fmt.Errorf("fail to write to local:%v, reason:%v", c, err))
				} else {
					ctx.Cancle(nil)
				}
			}()
		}
	}()

	return s
}

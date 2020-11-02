package internal

import (
	"crypto/tls"
	"io"
	"sync"
	"time"

	quic "github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/logging"
	"github.com/lucas-clemente/quic-go/qlog"
)

type quicConnectorBundle struct {
	connectoCtx     CanclableContext
	connect         string
	streamPerSesion int
	timeout         int
}

type quicConnectRequest struct {
	ctx       CanclableContext
	packet    QsockPacket
	pushReady func(io.ReadWriter)
}

func quicConnector(bundle quicConnectorBundle) (raceConnector, error) {
	connectorCtx := bundle.connectoCtx
	requests := make(chan connectBundle)
	go streamPoll(streamPollBundle{
		connectorCtx,
		requests,
		bundle.connect,
		bundle.streamPerSesion,
		time.Duration(bundle.timeout) * time.Second,
	})

	return func(connBundle connectBundle) {
		select {
		case requests <- connBundle:
		case <-connBundle.ctx.Done():
		case <-connectorCtx.Done():
			close(requests)
			connBundle.ctx.CancleWithError(connectorCtx.Err())
		}
	}, nil
}

type streamPollBundle struct {
	ctx             CanclableContext
	requests        chan connectBundle
	connect         string
	streamPerSesion int
	timeout         time.Duration
}

type tracer struct {
	p            logging.Perspective
	connectionID []byte
}

func (t tracer) Write(b []byte) (int, error) {
	LogTrace("p:%s connectionID:%v trace:%s", t.p, t.connectionID, b)
	return len(b), nil
}

func (t tracer) Close() error {
	return nil
}

func streamPoll(bundle streamPollBundle) {
	for {
		// build new sesion
		sessionCtx := bundle.ctx.Derive(nil)
		session, err := quic.DialAddrEarly(bundle.connect,
			&tls.Config{
				InsecureSkipVerify: true,
				NextProtos:         PeerQuicProtocol,
			},
			&quic.Config{
				Tracer: qlog.NewTracer(func(p logging.Perspective, connectionID []byte) io.WriteCloser {
					return tracer{
						p:            p,
						connectionID: connectionID,
					}
				}),
				HandshakeTimeout: bundle.timeout,
				MaxIdleTimeout:   bundle.timeout,
				KeepAlive:        true,
			},
		)
		if err != nil {
			// collect and retry
			sessionCtx.CancleWithError(err)
			continue
		}
		sessionCtx.Cleanup(func() error {
			session.CloseWithError(0, "")
			return nil
		})

		// limit streams per session
		wg := &sync.WaitGroup{}
		for i := 0; i < bundle.streamPerSesion; i++ {
			req, more := <-bundle.requests
			if !more {
				LogInfo("reqeust queue empty,quit connector")
				return
			}

			wg.Add(1)
			// async connect,
			// for faster concurrent raceConnector
			go func() {
				// open stream
				streamCtx := req.ctx
				streamCtx.Cleanup(func() error {
					wg.Done()
					return nil
				})

				// attach to session context,
				// since reqeust context is from raceConnector
				sessionCtx.Cleanup(func() error {
					streamCtx.CancleWithError(sessionCtx.Err())
					return nil
				})

				stream, err := session.OpenStream()
				if err != nil {
					// open stream fail,
					// maybe session broken,cancel it
					streamCtx.CancleWithError(err)
					return
				}
				streamCtx.Cleanup(stream.Close)

				LogInfo("quic connector -> %s:%d", req.addr, req.port)

				// write request
				packet := QsockPacket{
					0x01,
					req.port,
					req.addr,
				}
				if err := packet.Encode(stream); err != nil {
					streamCtx.CancleWithError(err)
					return
				}

				// establish
				req.pushReady(stream)
			}()
		}

		// to cleanup sesion
		go func() {
			wg.Wait()
			LogInfo("free up a quic session:%v", session)
			sessionCtx.Cancle()
		}()
	}
}

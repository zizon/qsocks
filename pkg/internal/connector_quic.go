package internal

import (
	"crypto/tls"
	"io"
	"sync"

	quic "github.com/lucas-clemente/quic-go"
)

type quicConnectorBundle struct {
	connectoCtx CanclableContext
	connect     string
	requests    chan connectBundle
}

type quicConnectRequest struct {
	ctx       CanclableContext
	packet    QsockPacket
	pushReady func(io.ReadWriter)
}

func quicConnector(bundle quicConnectorBundle) (raceConnector, error) {
	connectorCtx := bundle.connectoCtx

	go streamPoll(streamPollBundle{
		connectorCtx,
		bundle.requests,
		bundle.connect,
	})

	return raceConnectorFunc(func(connBundle connectBundle) {
		select {
		case bundle.requests <- connBundle:
		case <-connBundle.ctx.Done():
		case <-connectorCtx.Done():
			connBundle.ctx.CancleWithError(connectorCtx.Err())
		}
	}), nil
}

type streamPollBundle struct {
	ctx      CanclableContext
	requests chan connectBundle
	connect  string
}

func streamPoll(bundle streamPollBundle) {
	for {
		select {
		case <-bundle.ctx.Done():
			// context die
			return
		default:
		}

		// build new sesion
		sessionCtx := bundle.ctx.Derive(nil)
		session, err := quic.DialAddrContext(sessionCtx, bundle.connect, &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         PeerQuicProtocol,
		}, &quic.Config{})
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
		for i := 0; i < 10; i++ {
			// remember to do pushReady
			select {
			case <-sessionCtx.Done():
				// die,quit
				break
			case req := <-bundle.requests:
				// open stream
				streamCtx := req.ctx
				wg.Add(1)
				streamCtx.Cleanup(func() error {
					wg.Done()
					return nil
				})

				// async connect
				go func() {
					stream, err := session.OpenStreamSync(streamCtx)
					if err != nil {
						streamCtx.CancleWithError(err)
						return
					}
					sessionCtx.Cleanup(func() error {
						streamCtx.CancleWithError(sessionCtx.Err())
						return nil
					})
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
		}

		// to cleanup sesion
		go func() {
			wg.Wait()
			LogInfo("free up a quic session:v", session)
			sessionCtx.Cancle()
		}()
	}
}

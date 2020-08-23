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
}

type quicConnectRequest struct {
	packet    QsockPacket
	pushReady func(io.ReadWriter)
}

func quicConnector(bundle quicConnectorBundle) (raceConnector, error) {
	connectorCtx := bundle.connectoCtx

	requests := make(chan quicConnectRequest)
	go streamPoll(streamPollBundle{
		connectorCtx,
		requests,
		bundle.connect,
	})

	connector := raceConnectoable{
		connectFunc: func(connBundle connectBundle) {
			requests <- quicConnectRequest{
				QsockPacket{
					0x01,
					connBundle.port,
					connBundle.addr,
				},
				connBundle.pushReady,
			}
		},
		dropFunc: func(rw io.ReadWriter) {
			if stream, ok := rw.(quic.Stream); ok {
				if err := stream.Close(); err != nil {
					connectorCtx.CollectError(err)
				}
			}
		},
	}

	return connector, nil
}

type streamPollBundle struct {
	ctx      CanclableContext
	requests chan quicConnectRequest
	connect  string
}

func streamPoll(bundle streamPollBundle) {
	for {
		// build new sesion
		sessionCtx := bundle.ctx.Derive(nil)
		session, err := quic.DialAddrContext(sessionCtx, bundle.connect, &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         PeerQuicProtocol,
		}, &quic.Config{})
		if err != nil {
			bundle.ctx.CancleWithError(err)
			continue
		}
		sessionCtx.Cleanup(func() error {
			session.CloseWithError(0, "")
			return nil
		})

		// limit streams per session
		wg := &sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			select {
			case <-sessionCtx.Done():
				// parent die,quit
				sessionCtx.CancleWithError(bundle.ctx.Err())
				return
			case req := <-bundle.requests:
				// open stream
				streamCtx := sessionCtx.Derive(nil)

				wg.Add(1)
				stream, err := session.OpenStreamSync(sessionCtx)
				if err != nil {
					streamCtx.CancleWithError(err)
					continue
				}
				streamCtx.Cleanup(stream.Close)
				streamCtx.Cleanup(func() error {
					wg.Done()
					return nil
				})

				// write request
				if err := req.packet.Encode(stream); err != nil {
					streamCtx.CancleWithError(err)
					continue
				}

				// establish
				go req.pushReady(stream)
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

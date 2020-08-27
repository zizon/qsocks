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
	ctx      CanclableContext
	requests chan connectBundle
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
			//
			req, more := <-bundle.requests
			if !more {
				LogInfo("reqeust queue empty,quit connector")
				return
			}

			// async connect,
			// for faster concurrent raceConnector
			go func() {
				// open stream
				streamCtx := req.ctx
				wg.Add(1)
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

				stream, err := session.OpenStreamSync(streamCtx)
				if err != nil {
					// open stream fail,
					// maybe session broken,cancel it
					sessionCtx.CancleWithError(err)
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
			LogInfo("free up a quic session:v", session)
			sessionCtx.Cancle()
		}()
	}
}

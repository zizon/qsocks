package internal

import (
	"crypto/tls"
	"io"
	"reflect"
	"sync"

	quic "github.com/lucas-clemente/quic-go"
)

type quicConnectorBundle struct {
	connectoCtx CanclableContext
	connect     string
	requests    chan quicConnectRequest
}

type quicConnectRequest struct {
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

	connector := raceConnectoable{
		connectFunc: func(connBundle connectBundle) {
			select {
			case bundle.requests <- quicConnectRequest{
				QsockPacket{
					0x01,
					connBundle.port,
					connBundle.addr,
				},
				connBundle.pushReady,
			}:

			case <-connectorCtx.Done():
				// notify finished
				connBundle.ctx.Cancle()
			}
		},
		dropFunc: func(rw io.ReadWriter) {
			if stream, ok := rw.(quic.Stream); ok {
				if err := stream.Close(); err != nil {
					connectorCtx.CollectError(err)
				}
			} else {
				LogWarn("expect type quic stream,but got:%v", reflect.TypeOf(rw))
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
				streamCtx := sessionCtx.Derive(nil)
				stream, err := session.OpenStreamSync(streamCtx)
				if err != nil {
					streamCtx.CancleWithError(err)
					continue
				}
				wg.Add(1)
				streamCtx.Cleanup(func() error {
					wg.Done()
					return stream.Close()
				})
				LogInfo("quic connector -> %s:%d", req.packet.HOST, req.packet.PORT)

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

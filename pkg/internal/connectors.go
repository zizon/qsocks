package internal

import (
	"crypto/tls"
	"fmt"
	"io"
	"sync"

	quic "github.com/lucas-clemente/quic-go"
)

type quicConnectorBundle struct {
	connectoCtx CanclableContext
	addr        string
}

type quicConnectRequest struct {
	packet    QsockPacket
	pushReady func(io.ReadWriter)
}

func quicConnector(bundle quicConnectorBundle) (raceConnector, error) {
	connectorCtx := bundle.connectoCtx
	s, err := quic.DialAddrContext(connectorCtx, bundle.addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         PeerQuicProtocol,
	}, &quic.Config{})
	if err != nil {
		return nil, err
	}
	connectorCtx.Cleanup(func() error {
		s.CloseWithError(0, "")
		return nil
	})

	requests := make(chan quicConnectRequest)
	go func() {
		// stream limit for each session
		wg := &sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			wg.Add(1)
			connCtx := connectorCtx.Derive(nil)
			connCtx.Cleanup(func() error {
				LogInfo("a stream closed")
				wg.Done()
				return nil
			})

			// 1. pull request
			request, more := <-requests
			if !more {
				connectorCtx.CancleWithError(fmt.Errorf("no more request receive by quic connector:%v", bundle))
				break
			}

			// 2. connect one
			stream, err := s.OpenStreamSync(connCtx)
			if err != nil {
				connectorCtx.CancleWithError(err)
				continue
			}
			connectorCtx.Cleanup(stream.Close)

			// 3. handshake
			if err := request.packet.Encode(stream); err != nil {
				connectorCtx.CancleWithError(err)
				continue
			}

			// push to streams
			request.pushReady(stream)
		}

		// free up session
		go func() {
			wg.Wait()
			LogInfo("freeup session:%v", s)
			connectorCtx.Cancle()
		}()
	}()

	connector := raceConnectoable{
		connectFunc: func(connBundle connectBundle) {
			//TODO
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

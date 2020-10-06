package internal

import (
	"context"
	"crypto/tls"
	"io"

	quic "github.com/lucas-clemente/quic-go"
)

type blindBundle struct {
	serverCtx CanclableContext
	listen    string
	forward   string
}

// StartBlindServer start blind peer to peer forwarding server
func StartBlindServer(ctx context.Context, listen string, forward string) CanclableContext {
	serverCtx := NewCanclableContext(ctx)

	go blindServer(blindBundle{
		serverCtx: serverCtx,
		listen:    listen,
		forward:   forward,
	})

	return serverCtx
}

func blindServer(bundle blindBundle) {
	server, err := quic.ListenAddr(bundle.listen, generateTLSConfig(), nil)
	if err != nil {
		bundle.serverCtx.CancleWithError(err)
		return
	}
	bundle.serverCtx.Cleanup(server.Close)
	LogInfo("quic server listening:%s", server.Addr())

	for {
		session, err := server.Accept(bundle.serverCtx)
		if err != nil {
			bundle.serverCtx.CancleWithError(err)
			return
		}
		LogInfo("start quic server session:%v\n", session)

		sessionCtx := bundle.serverCtx.Derive(nil)
		sessionCtx.Cleanup(func() error {
			session.CloseWithError(0, "")
			return nil
		})

		go serveSession(sessionBundle{
			sessionCtx: sessionCtx,
			session:    session,
			forward:    bundle.forward,
		})
	}
}

type sessionBundle struct {
	sessionCtx CanclableContext
	session    quic.Session
	forward    string
}

func serveSession(bundle sessionBundle) {
	localSession := bundle.session
	ctx := bundle.sessionCtx

	// create remote session
	remoteSession, err := quic.DialAddrEarly(bundle.forward, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         PeerQuicProtocol,
	}, &quic.Config{})
	if err != nil {
		ctx.CancleWithError(err)
		return
	}
	ctx.Cleanup(func() error {
		remoteSession.CloseWithError(0, "")
		return nil
	})

	for {
		from, err := localSession.AcceptStream(ctx)
		if err != nil {
			ctx.CancleWithError(err)
			return
		}

		// prepree sub context
		streamCtx := ctx.Derive(nil)
		streamCtx.Cleanup(from.Close)

		go func() {
			// connect remote
			to, err := remoteSession.OpenStream()
			if err != nil {
				streamCtx.CancleWithError(err)
				return
			}

			// do copy
			go BiCopy(streamCtx, from, to, io.Copy)
		}()
	}
}

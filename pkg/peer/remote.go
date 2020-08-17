package peer

import (
	"crypto/tls"
	"io"

	quic "github.com/lucas-clemente/quic-go"
)

var (
	// PeerQuicProtocol quic peer protocl
	PeerQuicProtocol = []string{"quic-peer"}
)

// RemotePeer remote peer fo quic channel
type RemotePeer interface {
	// create new channel to remote
	NewChannel() (io.ReadWriteCloser, error)
}

// RemotePeerConfig config for create new remote peer
type RemotePeerConfig struct {
	quic.Config
	Addr string
}

type remotePeer struct {
	quic.Session
	CanclableContext
}

// NewRemotePeer create new remote peer
func NewRemotePeer(ctx CanclableContext, config RemotePeerConfig) (RemotePeer, error) {
	rCtx := ctx.Derive(nil)
	session, err := quic.DialAddrContext(
		rCtx, config.Addr,
		&tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         PeerQuicProtocol,
		},
		&config.Config)
	if err != nil {
		return nil, err
	}

	rCtx.Cleancup(func() {
		if err := session.CloseWithError(0, "closed"); err != nil {
			rCtx.CollectError(err)
		}
	})

	return remotePeer{
		session,
		ctx,
	}, nil
}

func (peer remotePeer) NewChannel() (io.ReadWriteCloser, error) {
	stream, err := peer.OpenStreamSync(peer)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

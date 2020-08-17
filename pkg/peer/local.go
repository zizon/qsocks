package peer

import (
	"io"
	"net"
)

// LocalPeer for local enpoint
type LocalPeer interface {
	PollNewChannel() (io.ReadWriteCloser, error)
}

// LocalPeerConfig config for local peer
type LocalPeerConfig struct {
	Addr string
}

type localPeer struct {
	CanclableContext
	net.Listener
}

// NewLocalPeer create new local endpoint
func NewLocalPeer(ctx CanclableContext, config LocalPeerConfig) (LocalPeer, error) {
	listener, err := net.Listen("tcp", config.Addr)
	if err != nil {
		return nil, err
	}

	peerCtx := ctx.Derive(nil)
	peerCtx.Cleancup(func() {
		if err := listener.Close(); err != nil {
			peerCtx.CollectError(err)
		}
	})

	return localPeer{
		ctx,
		listener,
	}, nil
}

func (peer localPeer) PollNewChannel() (io.ReadWriteCloser, error) {
	return peer.Accept()
}

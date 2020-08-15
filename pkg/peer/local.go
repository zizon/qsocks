package peer

import (
	"context"
	"io"
	"net"
)

// LocalPeer for local enpoint
type LocalPeer interface {
	io.Closer
	PollNewChannel() (io.ReadWriteCloser, error)
}

// LocalPeerConfig config for local peer
type LocalPeerConfig struct {
	Addr string
	ContextErrorAggregator
}

type localPeer struct {
	context.Context
	net.Listener
}

// NewLocalPeer create new local endpoint
func NewLocalPeer(ctx context.Context, config LocalPeerConfig) (LocalPeer, error) {
	listener, err := net.Listen("tcp", config.Addr)
	if err != nil {
		return nil, err
	}

	// associate context
	go func() {
		if block := ctx.Done(); block != nil {
			<-block
			if err := listener.Close(); err != nil {
				config.Collect(err)
			}
		}
	}()

	return &localPeer{
		ctx,
		listener,
	}, nil
}

func (peer localPeer) PollNewChannel() (io.ReadWriteCloser, error) {
	return peer.Accept()
}

func (peer localPeer) Close() error {
	return peer.Close()
}

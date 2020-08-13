package peer

import (
	"context"
	"io"
	"net"
)

// LocalPeer for local enpoint
type LocalPeer interface {
	io.Closer
	PollNewChannel(ctx context.Context) (io.ReadWriteCloser, error)
}

// LocalPeerConfig config for local peer
type LocalPeerConfig struct {
	addr string
}

type localPeer struct {
	listener net.Listener
}

// NewLocalPeer create new local endpoint
func NewLocalPeer(ctx context.Context, config LocalPeerConfig) (LocalPeer, error) {
	listener, err := net.Listen("tcp", config.addr)
	if err != nil {
		return nil, err
	}

	return &localPeer{
		listener: listener,
	}, nil
}

func (peer localPeer) PollNewChannel(ctx context.Context) (io.ReadWriteCloser, error) {
	return peer.listener.Accept()
}

func (peer localPeer) Close() error {
	return peer.Close()
}

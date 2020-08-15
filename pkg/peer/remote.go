package peer

import (
	"context"
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
	io.Closer
	// create new channel to remote
	NewChannel() (io.ReadWriteCloser, error)
}

// RemotePeerConfig config for create new remote peer
type RemotePeerConfig struct {
	quic.Config
	Addr string
	ContextErrorAggregator
}

type remotePeer struct {
	quic.Session
	context.Context
}

// NewRemotePeer create new remote peer
func NewRemotePeer(ctx context.Context, config RemotePeerConfig) (RemotePeer, error) {
	session, err := quic.DialAddrContext(
		ctx, config.Addr,
		&tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         PeerQuicProtocol,
		},
		&config.Config)
	if err != nil {
		return nil, err
	}

	go func() {
		if block := ctx.Done(); block != nil {
			<-block
			if err := session.CloseWithError(0, "closed"); err != nil {
				config.Collect(err)
			}
		}
	}()

	return &remotePeer{
		session,
		ctx,
	}, nil
}

func (peer remotePeer) NewChannel() (io.ReadWriteCloser, error) {
	stream, err := peer.OpenStreamSync(peer.Context)
	if err != nil {
		return nil, err
	}

	return stream, nil
}
func (peer remotePeer) Close() error {
	return peer.Close()
}

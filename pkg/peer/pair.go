package peer

import (
	"context"
	"io"
)

// PairConfig config for paring
type PairConfig struct {
	LocalPeerConfig
	RemotePeerConfig
	ContextErrorAggregator
}

type pairing struct {
	context.Context
	LocalPeer
	RemotePeer
	ContextErrorAggregator
}

// Pair pare local and remote peer
func Pair(pairCtx context.Context, pairConfig PairConfig) error {
	localPeer, err := NewLocalPeer(pairCtx, pairConfig.LocalPeerConfig)
	if err != nil {
		return err
	}

	remotePeer, err := NewRemotePeer(pairCtx, pairConfig.RemotePeerConfig)
	if err != nil {
		return err
	}

	paring := pairing{
		pairCtx,
		localPeer,
		remotePeer,
		pairConfig,
	}

	go paring.serve()

	go func() {
		if block := pairCtx.Done(); block != nil {
			defer paring.Close()
			<-block
		}
	}()
	return nil
}

func (peer pairing) Close() {
	if err := peer.LocalPeer.Close(); err != nil {
		peer.Collect(err)
	}

	if err := peer.RemotePeer.Close(); err != nil {
		peer.Collect(err)
	}
}

func (peer pairing) serve() {
	defer peer.Close()

	localPeer := peer.LocalPeer
	remotePeer := peer.RemotePeer
	for {
		// for local endpoint
		in, err := localPeer.PollNewChannel()
		if err != nil {
			peer.Collect(err)
			break
		}

		// for remote endpoint
		out, err := remotePeer.NewChannel()
		if err != nil {
			peer.Collect(err)

			// close local
			if err = in.Close(); err != nil {
				peer.Collect(err)
			}

			continue
		}

		// tunnel
		go func() {
			defer func() {
				if err := in.Close(); err != nil {
					peer.Collect(err)
				}

				if err := out.Close(); err != nil {
					peer.Collect(err)
				}
			}()

			// TODO
			_, err := io.Copy(out, in)
			if err != nil {
				peer.Collect(err)
			}
		}()
	}
}

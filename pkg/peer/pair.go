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
	context.CancelFunc
}

// Pair pare local and remote peer
func Pair(ctx context.Context, pairConfig PairConfig) error {
	// have derived goroutine,make new root context
	pairCtx, cancler := context.WithCancel(ctx)

	// calibrate error collector
	pairConfig.LocalPeerConfig.ContextErrorAggregator = pairConfig.ContextErrorAggregator
	pairConfig.RemotePeerConfig.ContextErrorAggregator = pairConfig.ContextErrorAggregator

	// listener local
	localPeer, err := NewLocalPeer(pairCtx, pairConfig.LocalPeerConfig)
	if err != nil {
		cancler()
		return err
	}

	// connect remote
	remotePeer, err := NewRemotePeer(pairCtx, pairConfig.RemotePeerConfig)
	if err != nil {
		cancler()
		return err
	}

	// prepare
	pairing := pairing{
		pairCtx,
		localPeer,
		remotePeer,
		pairConfig,
		cancler,
	}

	// go serve
	go pairing.serve()

	go func() {
		if block := pairCtx.Done(); pairCtx != nil {
			<-block
			cancler()

			// since resource are managed by local & remote peer
			// which associated with pairCtx, so just calling cancler
			// should be sufficient to clanup usage
		}
	}()

	return nil
}

func (peer pairing) serve() {
	defer peer.CancelFunc()

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

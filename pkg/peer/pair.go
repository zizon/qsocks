package peer

import (
	"context"
	"io"
)

type PairConfig struct {
	LocalPeerConfig
	RemotePeerConfig
	Collector ContextErrorAggregator
}

// Pair pare local and remote peer
func Pair(ctx context.Context, pairConfig PairConfig) (context.CancelFunc, error) {
	pairCtx, canceler := context.WithCancel(ctx)
	localPeer, err := NewLocalPeer(pairCtx, pairConfig.LocalPeerConfig)
	if err != nil {
		return nil, err
	}

	remotePeer, err := NewRemotePeer(pairCtx, pairConfig.RemotePeerConfig)
	if err != nil {
		return nil, err
	}

	go serve(pairContext{
		pairCtx,
		localPeer,
		remotePeer,
		pairConfig.Collector,
	})

	return canceler, nil
}

type pairContext struct {
	context.Context
	localPeer  LocalPeer
	remotePeer RemotePeer
	collector  ContextErrorAggregator
}

func serve(ctx pairContext) {
	pairCtx := ctx.Context
	localPeer := ctx.localPeer
	remotePeer := ctx.remotePeer
	collector := ctx.collector

	defer func() {
		if err := localPeer.Close(); err != nil {
			collector.Collect(err)
		}

		if err := remotePeer.Close(); err != nil {
			collector.Collect(err)
		}
	}()

	for {
		select {
		case <-pairCtx.Done():
			if pairCtx.Err() != nil {
				collector.Collect(pairCtx.Err())
			}

			break
		default:
		}

		// for local endpoint
		in, err := localPeer.PollNewChannel(pairCtx)
		if err != nil {
			collector.Collect(err)
			break
		}

		// for remote endpoint
		out, err := remotePeer.NewChannel(pairCtx)
		if err != nil {
			collector.Collect(err)

			// close local
			if err = in.Close(); err != nil {
				collector.Collect(err)
			}

			continue
		}

		// tunnel
		go func() {
			defer func() {
				if err := in.Close(); err != nil {
					collector.Collect(err)
				}

				if err := out.Close(); err != nil {
					collector.Collect(err)
				}
			}()

			// TODO
			_, err := io.Copy(out, in)
			if err != nil {
				collector.Collect(err)
			}
		}()
	}
}

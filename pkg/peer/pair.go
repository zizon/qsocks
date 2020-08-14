package peer

import (
	"context"
	"io"
)

// PairListener to receive various status
type PairListener interface {
	OnLocalEnpointError(error)
	OnRemoteEnpointError(error)
	OnTunnuelingError(error)
	OnTunnelEnd(int64)
}

type pairContext struct {
	context.Context
	localPeer  LocalPeer
	remotePeer RemotePeer
	listener   PairListener
}

// Pair pare local and remote peer
func Pair(ctx context.Context, local LocalPeerConfig, remote RemotePeerConfig, listener PairListener) (context.CancelFunc, error) {
	pairCtx, canceler := context.WithCancel(ctx)
	localPeer, err := NewLocalPeer(pairCtx, local)
	if err != nil {
		return nil, err
	}

	remotePeer, err := NewRemotePeer(pairCtx, remote)
	if err != nil {
		return nil, err
	}

	go serve(pairContext{
		pairCtx,
		localPeer,
		remotePeer,
		listener,
	})

	return canceler, nil
}

func serve(ctx pairContext) {
	pairCtx := ctx.Context
	localPeer := ctx.localPeer
	remotePeer := ctx.remotePeer
	listener := ctx.listener

	defer func() {
		if err := localPeer.Close(); err != nil {
			listener.OnLocalEnpointError(err)
		}

		if err := remotePeer.Close(); err != nil {
			listener.OnRemoteEnpointError(err)
		}
	}()

	for {
		select {
		case <-pairCtx.Done():
			if pairCtx.Err() != nil {
				listener.OnTunnuelingError(pairCtx.Err())
			}

			break
		default:
		}

		// for local endpoint
		in, err := localPeer.PollNewChannel(pairCtx)
		if err != nil {
			listener.OnLocalEnpointError(err)
			break
		}

		// for remote endpoint
		out, err := remotePeer.NewChannel(pairCtx)
		if err != nil {
			listener.OnRemoteEnpointError(err)

			// close local
			if err = in.Close(); err != nil {
				listener.OnRemoteEnpointError(err)
			}

			continue
		}

		// tunnel
		go func() {
			defer func() {
				if err := in.Close(); err != nil {
					listener.OnLocalEnpointError(err)
				}

				if err := out.Close(); err != nil {
					listener.OnRemoteEnpointError(err)
				}
			}()

			n, err := io.Copy(out, in)
			if err != nil {
				listener.OnTunnuelingError(err)
			}

			listener.OnTunnelEnd(n)
		}()
	}
}

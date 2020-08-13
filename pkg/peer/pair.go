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

// Pair pare local and remote peer
func Pair(ctx context.Context, local LocalPeerConfig, remote RemotePeerConfig, listener PairListener) context.CancelFunc {
	pairCtx, canceler := context.WithCancel(ctx)
	localPeer, err := NewLocalPeer(pairCtx, local)
	if err != nil {
		listener.OnLocalEnpointError(err)
		return canceler
	}

	remotePeer, err := NewRemotePeer(pairCtx, remote)
	if err != nil {
		listener.OnRemoteEnpointError(err)
		return canceler
	}

	for {
		select {
		case <-pairCtx.Done():
			if err := localPeer.Close(); err != nil {
				listener.OnLocalEnpointError(err)
			}

			if err = remotePeer.Close(); err != nil {
				listener.OnRemoteEnpointError(err)
			}
			break
		}

		// for local endpoint
		in, err := localPeer.PollNewChannel(pairCtx)
		if err != nil {
			listener.OnLocalEnpointError(err)
			continue
		}

		// for remote endpoint
		out, err := remotePeer.NewChannel(pairCtx)
		if err != nil {
			listener.OnRemoteEnpointError(err)

			if err = in.Close(); err != nil {
				listener.OnRemoteEnpointError(err)
			}

			continue
		}

		// tunnel
		go func() {
			n, err := io.Copy(out, in)
			if err != nil {
				listener.OnTunnuelingError(err)
			}

			listener.OnTunnelEnd(n)
		}()
	}

	return canceler
}

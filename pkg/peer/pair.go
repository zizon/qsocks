package peer

import (
	"io"
)

// PairConfig config for paring
type PairConfig struct {
	LocalPeerConfig
	RemotePeerConfig
}

type pairing struct {
	CanclableContext
	LocalPeer
	RemotePeer
}

// Pair pare local and remote peer
func Pair(ctx CanclableContext, pairConfig PairConfig) error {
	// have derived goroutine,make new root context
	pairCtx := ctx.Derive(nil)

	// listener local
	localPeer, err := NewLocalPeer(pairCtx, pairConfig.LocalPeerConfig)
	if err != nil {
		pairCtx.Cancle()
		return err
	}

	// connect remote
	remotePeer, err := NewRemotePeer(pairCtx, pairConfig.RemotePeerConfig)
	if err != nil {
		pairCtx.Cancle()
		return err
	}

	// go serve
	go pairing{
		pairCtx,
		localPeer,
		remotePeer,
	}.serve()

	return nil
}

func (peer pairing) serve() {
	defer peer.Close()

	localPeer := peer.LocalPeer
	remotePeer := peer.RemotePeer
	for {
		// for local endpoint
		in, err := localPeer.PollNewChannel()
		if err != nil {
			peer.CollectError(err)
			break
		}

		// for remote endpoint
		out, err := remotePeer.NewChannel()
		if err != nil {
			peer.CollectError(err)

			// close local
			if err = in.Close(); err != nil {
				peer.CollectError(err)
			}

			continue
		}

		ioCtx := peer.Derive(nil)
		go BiCopy(ioCtx, in, out)

		ioCtx.Cleancup(func() {
			if err := in.Close(); err != nil {
				peer.CollectError(err)
			}

			if err := out.Close(); err != nil {
				peer.CollectError(err)
			}
		})
	}
}

// BiCopy copy bidirectional for both first and second
func BiCopy(ctx CanclableContext, first io.ReadWriter, second io.ReadWriter) {
	// copy first -> second
	go func() {
		if _, err := io.Copy(second, first); err != nil {
			ctx.CollectError(err)
		}
	}()

	// copy second -> first
	go func() {
		if _, err := io.Copy(first, second); err != nil {
			ctx.CollectError(err)
		}
	}()
}

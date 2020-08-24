package internal

import (
	"io"
	"reflect"
)

type raceConnector func(connectBundle)

type connectBundle struct {
	ctx       CanclableContext
	pushReady func(io.ReadWriter)
	addr      string
	port      int
}

type raceBundle struct {
	ctx        CanclableContext
	connectors []raceConnector
	addr       string
	port       int
}

func receConnect(bundle raceBundle) io.ReadWriter {
	ready := make(chan io.ReadWriter)

	raceCtx := bundle.ctx.Derive(nil)
	raceCtx.Cleanup(func() error {
		close(ready)
		return nil
	})
	// go race connect
	for _, connector := range bundle.connectors {
		// raceCtx are used for syncronize,
		// do not attatch to it, or connection
		// may be drop immediatly
		connectCtx := bundle.ctx.Derive(nil)

		go connector(connectBundle{
			connectCtx,
			func(rw io.ReadWriter) {
				defer raceCtx.Cancle()

				// wait for ready or die
				select {
				case ready <- rw:
					// first ready connection wins
					LogInfo("win race connect -> %s:%d %v", bundle.addr, bundle.port, reflect.TypeOf(rw))
					return
				case <-raceCtx.Done():
					connectCtx.CancleWithError(raceCtx.Err())
					return
				}

			},
			bundle.addr,
			bundle.port,
		})
	}

	// pull first ready
	select {
	case rw := <-ready:
		return rw
	case <-bundle.ctx.Done():
		return nil
	}
}

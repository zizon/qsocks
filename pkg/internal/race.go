package internal

import (
	"io"
	"reflect"
)

type raceConnector interface {
	connect(connectBundle)
	drop(io.ReadWriter)
}

type raceConnectoable struct {
	connectFunc func(connectBundle)
	dropFunc    func(io.ReadWriter)
}

func (c raceConnectoable) connect(bundle connectBundle) {
	if c.connectFunc != nil {
		c.connectFunc(bundle)
		return
	}
	LogWarn("no connect function for:%v", bundle)
}

func (c raceConnectoable) drop(rw io.ReadWriter) {
	if c.dropFunc != nil {
		c.dropFunc(rw)
		return
	}
	LogWarn("no dropFunc function for:%v", rw)
}

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
	// go race connect
	for _, connector := range bundle.connectors {
		// raceCtx are used for syncronize,
		// do not attatch to it, or connection
		// may be drop immediatly
		connectCtx := bundle.ctx.Derive(nil)

		go connector.connect(connectBundle{
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
					// block in push,as some had already push
					// wait notify and do cleanup,
					return
				}

			},
			bundle.addr,
			bundle.port,
		})
	}

	// pull first ready
	rw := <-ready

	// cancle the other
	go func() {
		raceCtx.Cancle()
		close(ready)
	}()

	return rw
}

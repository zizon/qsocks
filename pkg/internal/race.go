package internal

import (
	"io"
	"sync"
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
	}
}

func (c raceConnectoable) drop(rw io.ReadWriter) {
	if c.dropFunc != nil {
		c.dropFunc(rw)
	}
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
	wg := &sync.WaitGroup{}

	ready := make(chan io.ReadWriter)

	raceCtx := bundle.ctx.Derive(nil)
	// go race connect
	for _, connector := range bundle.connectors {
		wg.Add(1)
		connectCtx := raceCtx.Derive(nil)
		once := &sync.Once{}
		go connector.connect(connectBundle{
			connectCtx,
			func(connectorReady io.ReadWriter) {
				once.Do(func() {
					defer wg.Done()
					select {
					case ready <- connectorReady:
						// first ready connection wins
						return
					case <-raceCtx.Done():
						// block in push,as some had already push
						// wait notify and do cleanup,
						connectCtx.Cancle()
						return
					}
				})
			},
			bundle.addr,
			bundle.port,
		})

		// glue under reaceCtx
		raceCtx.Cleanup(func() error {
			connectCtx.Cancle()
			return nil
		})
	}

	// poll firest conncted
	rw := <-ready

	// then start close ready channel
	go func() {
		// pushReady are guarded by once,
		// so, wg should be in correct behavior, and close ready channel should be fine
		wg.Wait()
		close(ready)
	}()

	return rw
}

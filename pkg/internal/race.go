package internal

import (
	"io"
	"reflect"
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
	wg := &sync.WaitGroup{}

	ready := make(chan io.ReadWriter)

	raceCtx := bundle.ctx.Derive(nil)
	bundle.ctx.Cleanup(func() error {
		raceCtx.Cancle()
		return nil
	})
	// go race connect
	for _, connector := range bundle.connectors {
		wg.Add(1)

		// raceCtx are used for syncronize,
		// do not attatch to it, or connection will
		// be drop immediatly
		connectCtx := bundle.ctx.Derive(nil)
		once := &sync.Once{}
		go connector.connect(connectBundle{
			connectCtx,
			func(rw io.ReadWriter) {
				once.Do(func() {
					// raced one
					defer wg.Done()

					// wati for ready or die
					select {
					case ready <- rw:
						// first ready connection wins
						LogInfo("win race connect -> %s:%d %v", bundle.addr, bundle.port, reflect.TypeOf(rw))
						raceCtx.Cancle()
						return
					case <-raceCtx.Done():
						// block in push,as some had already push
						// wait notify and do cleanup,
						return
					}
				})
			},
			bundle.addr,
			bundle.port,
		})
	}

	// start close ready channel
	go func() {
		// pushReady are guarded by once,
		// so, wg should be in correct behavior, and close ready channel should be fine
		wg.Wait()
		close(ready)
	}()

	// poll firest conncted
	// may be all connect ware fail?
	rw := <-ready

	return rw
}

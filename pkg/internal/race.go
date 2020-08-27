package internal

import (
	"io"
	"reflect"
	"sync"
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

type winner struct {
	rw    io.ReadWriter
	index int
}

func receConnect(bundle raceBundle) io.ReadWriter {
	once := &sync.Once{}
	ready := make(chan winner)
	connectCtxs := make([]CanclableContext, len(bundle.connectors))

	// go race connect
	for i, connector := range bundle.connectors {
		// raceCtx are used for syncronize,
		// do not attatch to it, or connection
		// may be drop immediatly
		connectCtx := bundle.ctx.Derive(nil)
		connectCtxs[i] = connectCtx

		go connector(connectBundle{
			connectCtx,
			func(tracker int) func(rw io.ReadWriter) {
				return func(rw io.ReadWriter) {
					once.Do(func() {
						ready <- winner{
							rw,
							tracker,
						}
						close(ready)
					})
				}
			}(i),
			bundle.addr,
			bundle.port,
		})
	}

	allDone := make(chan struct{})
	go func() {
		for _, ctx := range connectCtxs {
			<-ctx.Done()
		}
		close(allDone)
	}()

	// either some race wins or all ctx canceled
	select {
	case winning := <-ready:
		LogInfo("winning race: %v", reflect.TypeOf(winning.rw))
		go func() {
			for i, ctx := range connectCtxs {
				if i != winning.index {
					ctx.Cancle()
				}
			}
		}()
		return winning.rw
	case <-allDone:
		LogWarn("all race fail")
	}

	return nil
}

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
	defer close(ready)

	connectStates := make([]CanclableContext, len(bundle.connectors))
	// go race connect
	for i, connector := range bundle.connectors {
		// raceCtx are used for syncronize,
		// do not attatch to it, or connection
		// may be drop immediatly
		connectCtx := bundle.ctx.Derive(nil)
		connectStates[i] = connectCtx

		go connector(connectBundle{
			connectCtx,
			func(rw io.ReadWriter) {
				// wait for ready or die
				select {
				case ready <- rw:
					// first ready connection wins
					return
				case <-connectCtx.Done():
					// or someone had already call pushReady,
					// cancel so as to free up resources
					return
				}
			},
			bundle.addr,
			bundle.port,
		})
	}

	// 1. some race success
	// 2. all race faile
	// 3. race canceld
	for i, state := range connectStates {
		select {
		case rw := <-ready:
			LogInfo("win race connect -> %s:%d %v", bundle.addr, bundle.port, reflect.TypeOf(rw))
			// go cacenl the others
			// use i is ok,since no more modification?
			go func() {
				for j, ctx := range connectStates {
					if i != j {
						ctx.Cancle()
					}
				}
			}()

			return rw
		case <-state.Done():
			// this connect fail,
			// wait for next
			LogInfo("some race was cancel")
			continue
		case <-bundle.ctx.Done():
			// cancel as a whole
			LogInfo("race wase cancel as a whole")
			return nil
		}
	}

	LogWarn("all race fail")
	// all fail
	return nil
}

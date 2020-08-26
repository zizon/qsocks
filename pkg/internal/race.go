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

type connectorJoinGroup struct {
	ctx   CanclableContext
	ready chan io.ReadWriter
}

func receConnect(bundle raceBundle) io.ReadWriter {
	connectStates := make([]connectorJoinGroup, len(bundle.connectors))

	// go race connect
	for i, connector := range bundle.connectors {
		// raceCtx are used for syncronize,
		// do not attatch to it, or connection
		// may be drop immediatly
		connectCtx := bundle.ctx.Derive(nil)
		joinGroup := connectorJoinGroup{
			connectCtx,
			make(chan io.ReadWriter),
		}
		connectStates[i] = joinGroup
		joinGroup.ctx.Cleanup(func() error {
			close(joinGroup.ready)
			return nil
		})

		go connector(connectBundle{
			connectCtx,
			func(rw io.ReadWriter) {
				select {
				case <-joinGroup.ctx.Done():
					return
				default:
				}

				// wait for ready or die
				select {
				case joinGroup.ready <- rw:
					LogInfo("win race connect -> %s:%d %v", bundle.addr, bundle.port, reflect.TypeOf(rw))
					// first ready connection wins
					return
				case <-connectCtx.Done():
					LogInfo("cancel race connect -> %s:%d %v", bundle.addr, bundle.port, reflect.TypeOf(rw))
					// or someone had already call pushReady,
					// cancel so as to free up resources.
					return
				}
			},
			bundle.addr,
			bundle.port,
		})
	}

	// 1. some race success
	// 2. all race faild/canceld
	for i, state := range connectStates {
		select {
		case rw := <-state.ready:
			go func() {
				for j, pending := range connectStates[i:] {
					if i != j {
						pending.ctx.Cancle()
					}
				}
			}()

			return rw
		case <-state.ctx.Done():
			// this connect fail,
			// wait for next
			LogInfo("some race was cancel")
			continue
		}
	}

	LogWarn("all race fail")
	// all fail
	return nil
}

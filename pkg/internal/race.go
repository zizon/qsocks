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
				// early test, since a cancled context means
				// closed ready channel, thus casing the second select
				// a chance to go reusme with the send case which
				// eventualy case panic
				select {
				case <-connectCtx.Done():
					return
				default:
				}

				// wait for ready or die
				select {
				case ready <- rw:
					// first ready connection wins
					return
				case <-connectCtx.Done():
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
		case rw := <-ready:
			LogInfo("win race connect -> %s:%d %v", bundle.addr, bundle.port, reflect.TypeOf(rw))

			// go cacenl the others
			// use i is ok,since no more modification?
			go func() {
				for j, ctx := range connectStates[i:] {
					if i != j {
						// trigger release reference of context life cycle
						ctx.Cancle()

						// ensure canceled
						<-ctx.Done()

						// since ensured canceld,
						// select case in pushReady should had been selected of
						// <-ctx.Done(), not channel select.
						// thus unblocking the select.
						// when another pushReady invoke, the early check should
						// safely avoid send on closed channel
					}
				}

				// there are one more condidtion.
				// a seconday race connect enter send channel before getting canceld.
				// so, optionlay pull that one
				select {
				case <-ready:
				default:
				}

				// should be safe to close ready,
				// since it should break selelet in race connect pushReady
				close(ready)
			}()

			return rw
		case <-state.Done():
			// this connect fail,
			// wait for next
			LogInfo("some race was cancel")
			continue
		}
	}

	LogWarn("all race fail")
	go func() {
		// race fail means all blocking of pushReady of case <-ctx.Done()
		// should had return. ensure it
		for _, state := range connectStates {
			<-state.Done()
		}

		// similly to race success, a optional seconday may had enter send channel.
		// pull it maybe
		select {
		case <-ready:
		default:
		}

		close(ready)
	}()

	// all fail
	return nil
}

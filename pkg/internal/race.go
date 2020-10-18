package internal

import (
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"
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
	timeout    int
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
	cases := make([]reflect.SelectCase, 0)
	cases = append(cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ready),
	})
	cases = append(cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(allDone),
	})

	if bundle.timeout > 0 {
		timer := time.NewTimer(time.Duration(bundle.timeout) * time.Second)
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(timer.C),
			Send: reflect.ValueOf(timer),
		})
	}

	chosen, value, _ := reflect.Select(cases)
	switch chosen {
	case 0:
		winning, ok := value.Interface().(winner)
		if !ok {
			bundle.ctx.CancleWithError(fmt.Errorf("fail to cast:%s to io.ReadWriter", value))
			return nil
		}
		go func() {
			LogInfo("winning -> %s:%d %s", bundle.addr, bundle.port, reflect.TypeOf(winning.rw))
			for i, ctx := range connectCtxs {
				if i != winning.index {
					ctx.Cancle()
				}
			}
		}()

		return winning.rw
	case 1:
		LogWarn("all race fail")
		return nil
	case 2:
		timer, ok := cases[3].Send.Interface().(*time.Timer)
		if !ok {
			bundle.ctx.CancleWithError(fmt.Errorf("timeout but got no timer value:%v", timer))
			return nil
		}

		bundle.ctx.CancleWithError(fmt.Errorf("timeout race connect"))
		return nil
	default:
		bundle.ctx.CancleWithError(fmt.Errorf("selected unexpected value:%v", value))
		return nil
	}
}

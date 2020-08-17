package peer

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
)

// ContextErrorAggregator collect errors over context life time
type ContextErrorAggregator interface {
	Collect(error)
}

// CanclableContext a cancleable context
type CanclableContext interface {
	context.Context
	Close() error
	Cancle()
	CollectError(error)
	Derive(collector ContextErrorAggregator) CanclableContext
	Cleancup(func())
}

type cancleContext struct {
	context.Context
	context.CancelFunc
	ContextErrorAggregator
}

// NewCanclableContext create a cancleable context with optional nil error collector
func NewCanclableContext(ctx context.Context, collector ContextErrorAggregator) CanclableContext {
	cancleCtx, cancler := context.WithCancel(ctx)
	return cancleContext{
		cancleCtx,
		cancler,
		collector,
	}
}

func (ctx cancleContext) Cancle() {
	ctx.CancelFunc()
}

func (ctx cancleContext) Close() error {
	ctx.CancelFunc()
	<-ctx.Done()
	return ctx.Err()
}

func (ctx cancleContext) CollectError(err error) {
	if ctx.ContextErrorAggregator != nil {
		ctx.ContextErrorAggregator.Collect(err)
		return
	}

	switch err {
	case context.Canceled:
		return
	}

	buf := make([]byte, 1024)
	runtime.Stack(buf, false)
	fmt.Printf("collect error: %s:%v\n%s\n", reflect.TypeOf(err), err, buf)
}

func (ctx cancleContext) Derive(collector ContextErrorAggregator) CanclableContext {
	if collector != nil {
		return NewCanclableContext(ctx, collector)
	}
	return NewCanclableContext(ctx, ctx)
}

func (ctx cancleContext) Cleancup(cleanup func()) {
	go func() {
		<-ctx.Done()

		cleanup()
	}()
}

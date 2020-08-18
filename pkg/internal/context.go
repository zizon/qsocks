package internal

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
)

// CanclableContext a cancleable context
type CanclableContext interface {
	context.Context
	Cancle()
	CancleWithError(error)
	CollectError(error)
	Derive(func(error)) CanclableContext
	Cleanup(func() error)
}

type cancleContext struct {
	context.Context
	context.CancelFunc
	collectError func(error)
}

// NewCanclableContext create a cancleable context with optional nil error collector
func NewCanclableContext(ctx context.Context, collector func(error)) CanclableContext {
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

func (ctx cancleContext) CancleWithError(err error) {
	if err != nil {
		ctx.CollectError(err)
	}

	ctx.CancelFunc()
}

func (ctx cancleContext) CollectError(err error) {
	switch err {
	case context.Canceled:
		return
	}

	if ctx.collectError != nil {
		ctx.collectError(err)
		return
	}

	buf := make([]byte, 1024)
	runtime.Stack(buf, false)
	fmt.Printf("collect error: %s:%v\n%s\n", reflect.TypeOf(err), err, buf)
}

func (ctx cancleContext) Derive(collector func(error)) CanclableContext {
	return NewCanclableContext(ctx, ctx.collectError)
}

func (ctx cancleContext) Cleanup(cleanup func() error) {
	go func() {
		<-ctx.Done()
		if err := cleanup(); err != nil {
			ctx.CollectError(err)
		}
	}()
}

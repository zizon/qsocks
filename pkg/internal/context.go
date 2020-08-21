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
func NewCanclableContext(ctx context.Context) CanclableContext {
	cancleCtx, cancler := context.WithCancel(ctx)
	return cancleContext{
		cancleCtx,
		cancler,
		defaultCollector,
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

var (
	defaultCollector = func(err error) {
		buf := make([]byte, 1024)
		runtime.Stack(buf, false)
		fmt.Printf("collect error: %s:%v\n%s\n", reflect.TypeOf(err), err, buf)
	}
)

// SetDefaultCollector sets default error collector
func SetDefaultCollector(collector func(err error)) {
	if collector != nil {
		defaultCollector = collector
	}
}

func (ctx cancleContext) CollectError(err error) {
	if ctx.collectError != nil {
		ctx.collectError(err)
		return
	}

	defaultCollector(err)
}

func (ctx cancleContext) Derive(collector func(error)) CanclableContext {
	if collector == nil {
		collector = ctx.collectError
	}

	cancleCtx, cancler := context.WithCancel(ctx)
	return cancleContext{
		cancleCtx,
		cancler,
		collector,
	}
}

func (ctx cancleContext) Cleanup(cleanup func() error) {
	go func() {
		<-ctx.Done()
		if err := cleanup(); err != nil {
			ctx.CollectError(err)
		}
	}()
}

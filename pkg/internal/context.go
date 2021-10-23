package internal

import (
	"context"
	"sync"
)

// CanclableContext a cancleable context
type CanclableContext interface {
	context.Context
	Cancle(reason error)

	Fork() CanclableContext
	OnCancle(func(error))
}

type cancleContext struct {
	context.Context
	context.CancelFunc
	onCancles []func(error)
	*sync.Mutex
	reason error
}

// NewCanclableContext create a cancleable context with optional nil error collector
func NewCanclableContext(ctx context.Context) CanclableContext {
	cancleCtx, cancler := context.WithCancel(ctx)
	return &cancleContext{
		Context:    cancleCtx,
		Mutex:      &sync.Mutex{},
		CancelFunc: cancler,
	}
}

func (ctx *cancleContext) Cancle(reason error) {
	ctx.Lock()
	if ctx.reason != nil {
		ctx.Unlock()
		return
	}

	// then canceld
	ctx.CancelFunc()
	ctx.reason = func() error {
		if reason != nil {
			return reason
		}

		return ctx.Context.Err()
	}()
	// do callback
	move := ctx.onCancles
	ctx.onCancles = nil
	ctx.Unlock()

	// do callback
	for _, cb := range move {
		cb(ctx.reason)
	}
}

func (ctx *cancleContext) Err() error {
	return ctx.reason
}

func (ctx *cancleContext) Fork() CanclableContext {
	newCtx, cancle := context.WithCancel(ctx)
	return &cancleContext{
		Context:    newCtx,
		CancelFunc: cancle,
		Mutex:      &sync.Mutex{},
	}
}

func (ctx *cancleContext) OnCancle(cb func(error)) {
	ctx.Lock()
	if ctx.reason != nil {
		ctx.Unlock()
		cb(ctx.reason)
		return
	}

	if ctx.onCancles == nil {
		ctx.onCancles = []func(error){cb}
	} else {
		ctx.onCancles = append(ctx.onCancles, cb)
	}
	ctx.Unlock()
}

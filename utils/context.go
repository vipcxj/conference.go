package utils

import (
	"context"
	"sync"
	"time"
)

type autoCancelContext struct {
	ctx      context.Context
	children []context.Context
	start    bool
	cause    error
	mux      sync.Mutex
}

type AutoCancelContextSpawn = func() (ctx context.Context, done context.CancelCauseFunc)
type AutoCancelContextStart = func()

func WithAutoCancel(parent context.Context) (ctx context.Context, spawn AutoCancelContextSpawn, start AutoCancelContextStart) {
	parent, cancel_self := context.WithCancelCause(parent)
	autoCtx := &autoCancelContext{
		ctx: parent,
	}
	ctx = autoCtx
	spawn = func() (child context.Context, done context.CancelCauseFunc) {
		autoCtx.mux.Lock()
		defer autoCtx.mux.Unlock()
		child, cancel_child := context.WithCancelCause(autoCtx)
		autoCtx.children = append(autoCtx.children, child)
		done = func(cause error) {
			cancel_child(cause)
			autoCtx.mux.Lock()
			defer autoCtx.mux.Unlock()
			autoCtx.children = SliceRemoveByValue(autoCtx.children, false, child)
			if len(autoCtx.children) == 0 {
				if autoCtx.start {
					cancel_self(cause)
				} else if autoCtx.cause == nil {
					autoCtx.cause = cause
				}
			}
		}
		return
	}
	start = func() {
		autoCtx.mux.Lock()
		defer autoCtx.mux.Unlock()
		autoCtx.start = true
		if len(autoCtx.children) == 0 {
			cancel_self(autoCtx.cause)
		}
	}
	return
}

func (c *autoCancelContext) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

func (c *autoCancelContext) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *autoCancelContext) Err() error {
	return c.ctx.Err()
}

func (c *autoCancelContext) Value(key any) any {
	return c.ctx.Value(key)
}

type orContext struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
}

func (c *orContext) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

func (c *orContext) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *orContext) Err() error {
	return c.ctx.Err()
}

func (c *orContext) Value(key any) any {
	return c.ctx.Value(key)
}

func WithOrContext(ctxs ...context.Context) *orContext {
	ctx, cancel := context.WithCancelCause(context.Background())
	for _, c := range ctxs {
		context.AfterFunc(c, func() {
			cancel(context.Cause(c))
		})
	}
	return &orContext{
		ctx:    ctx,
		cancel: cancel,
	}
}

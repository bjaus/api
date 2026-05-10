package api

import (
	"context"
	"log/slog"
	"sync"
)

// Background registers fn to run after the current response has been sent.
// The context passed to fn is detached from the request — it is not cancelled
// when the client disconnects or the request times out. Use this for
// fire-and-forget work like sending notifications, writing audit logs, or
// invalidating caches.
//
// Calling Background outside a handler (or after the handler returns) is a
// no-op. Each task runs in its own goroutine; a panic is logged and does not
// affect other tasks or the server.
func Background(ctx context.Context, fn func(ctx context.Context)) {
	q, ok := ctx.Value(bgQueueKey{}).(*bgQueue)
	if !ok {
		return
	}
	q.mu.Lock()
	q.funcs = append(q.funcs, fn)
	q.mu.Unlock()
}

type bgQueueKey struct{}

type bgQueue struct {
	mu    sync.Mutex
	funcs []func(context.Context)
}

// withBackgroundQueue returns a context that carries an empty background
// queue. Handlers append tasks via Background; the framework drains the queue
// after the response is sent.
func withBackgroundQueue(ctx context.Context) (context.Context, *bgQueue) {
	q := &bgQueue{}
	return context.WithValue(ctx, bgQueueKey{}, q), q
}

// runBackgroundTasks launches each queued task in its own goroutine with a
// fresh background context. Panics are recovered and logged.
func runBackgroundTasks(q *bgQueue) {
	q.mu.Lock()
	funcs := q.funcs
	q.funcs = nil
	q.mu.Unlock()
	for _, fn := range funcs {
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("background task panicked", "panic", rec)
				}
			}()
			fn(context.Background())
		}()
	}
}

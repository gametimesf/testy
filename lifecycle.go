package testy

import (
	"context"
	"sync"
)

// Cleanup registers f to run after this test and its subtests, in last-in,
// first-out order. It runs on return, Fatal/FailNow, and panic. Register cleanup
// immediately after acquiring a resource. Cleanup is supported in Test and Run
// callbacks, not the legacy Before/After hooks.
//
// This optional capability leaves TestingT source-compatible with custom
// implementations. Custom runners must implement Cleanup(func()) to use it;
// unsupported implementations panic rather than silently leak resources.
func Cleanup(t TestingT, f func()) {
	t.Helper()
	owner, ok := t.(interface{ Cleanup(func()) })
	if !ok {
		panic("testy: TestingT does not support Cleanup")
	}
	owner.Cleanup(f)
}

// Context returns this test's context, canceled immediately before its cleanup
// callbacks run. Subtest contexts inherit their parent's cancellation. Cleanup
// that performs I/O must use its own bounded context. Like Cleanup, this is an
// optional capability for custom TestingT implementations and is unavailable in
// legacy Before/After hooks.
func Context(t TestingT) context.Context {
	owner, ok := t.(interface{ Context() context.Context })
	if !ok {
		panic("testy: TestingT does not support Context")
	}
	return owner.Context()
}

type lifecycle struct {
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	cleanups []func()
}

func newLifecycle(parent context.Context) *lifecycle {
	ctx, cancel := context.WithCancel(parent)
	return &lifecycle{ctx: ctx, cancel: cancel}
}

func (l *lifecycle) add(f func()) {
	if f == nil {
		panic("testy: nil cleanup")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanups = append(l.cleanups, f)
}

// Each callback defers the remainder so even panic or Goexit cannot abandon
// earlier registrations. Pop at execution time to support nested registration.
func (l *lifecycle) finish() {
	l.cancel()
	l.mu.Lock()
	if len(l.cleanups) == 0 {
		l.mu.Unlock()
		return
	}
	i := len(l.cleanups) - 1
	f := l.cleanups[i]
	l.cleanups = l.cleanups[:i]
	l.mu.Unlock()
	defer l.finish()
	f()
}

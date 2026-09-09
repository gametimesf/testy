package testy

import (
	"context"
	"testing"
)

// tWrapper wraps a real testing.T, because Run takes a concrete implementation.
type tWrapper struct {
	t         *testing.T
	lifecycle *lifecycle
}

var _ TestingT = (*tWrapper)(nil)

func (t tWrapper) Fail() {
	t.Helper()
	t.t.Fail()
}

func (t tWrapper) FailNow() {
	t.Helper()
	t.t.FailNow()
}

func (t tWrapper) Fatal(args ...interface{}) {
	t.Helper()
	t.t.Fatal(args...)
}

func (t tWrapper) Fatalf(format string, args ...interface{}) {
	t.Helper()
	t.t.Fatalf(format, args...)
}

func (t tWrapper) Errorf(format string, args ...interface{}) {
	t.Helper()
	t.t.Errorf(format, args...)
}

func (t tWrapper) Helper() {
	// this probably doesn't actually work right since the call stack is incorrect
	t.t.Helper()
}

func (t tWrapper) Log(args ...interface{}) {
	t.Helper()
	t.t.Log(args...)
}

func (t tWrapper) Logf(format string, args ...interface{}) {
	t.Helper()
	t.t.Logf(format, args...)
}

func (t tWrapper) Run(s string, tester Tester) bool {
	t.t.Helper()
	return t.t.Run(s, func(tt *testing.T) {
		t.t.Helper()
		parent := context.Background()
		if t.lifecycle != nil {
			parent = t.lifecycle.ctx
		}
		tester(newTWrapper(tt, parent))
	})
}

func (t tWrapper) Parallel() {
	t.t.Parallel()
}

func newTWrapper(t *testing.T, parent context.Context) tWrapper {
	l := newLifecycle(parent)
	if deadline, ok := t.Deadline(); ok {
		ctx, cancel := context.WithDeadline(l.ctx, deadline)
		l.ctx = ctx
		previousCancel := l.cancel
		l.cancel = func() { cancel(); previousCancel() }
	}
	t.Cleanup(l.finish)
	return tWrapper{t: t, lifecycle: l}
}

func (t tWrapper) Cleanup(f func()) {
	if t.lifecycle == nil {
		panic("testy: Cleanup is only supported in Test and Run callbacks")
	}
	t.lifecycle.add(f)
}

func (t tWrapper) Context() context.Context {
	if t.lifecycle == nil {
		panic("testy: Context is only supported in Test and Run callbacks")
	}
	return t.lifecycle.ctx
}

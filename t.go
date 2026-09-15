package testy

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

type t struct {
	lifecycle   *lifecycle
	name        string
	tester      Tester
	failed      bool
	skipped     bool
	skipReason  string
	msgs        []Msg
	subtests    chan<- subtest
	subtestDone <-chan bool
}

type subtest struct {
	name   string
	tester Tester
}

var _ TestingT = (*t)(nil)

// test returns whether this t is actually being used in a test. This is determined by the tester func being non-nil.
func (t *t) test() bool {
	return t.tester != nil
}

func (t *t) run() {
	defer func() {
		// catch panics and mark test as failed
		if err := recover(); err != nil {
			// not using Fatalf since we're already in the defer that would get run and we need to clean up the channel
			t.Errorf("panic: %+v", err)
			t.Fail()
		}
		close(t.subtests)
	}()

	defer t.lifecycle.finish()
	// Record a body panic before cleanup: a later Goexit must not mask it.
	defer t.recoverPanic()
	t.tester(t)
}

func (t *t) Fail() {
	t.failed = true
}

func (t *t) FailNow() {
	t.Fail()
	if t.test() {
		runtime.Goexit()
	} else {
		panic("before/after helper t failed")
	}
}

func (t *t) Fatal(args ...interface{}) {
	t.msgs = append(t.msgs, Msg{Msg: fmt.Sprintln(args...), Level: LevelError})
	t.FailNow()
}

func (t *t) Fatalf(format string, args ...interface{}) {
	t.msgs = append(t.msgs, Msg{Msg: fmt.Sprintf(format, args...), Level: LevelError})
	t.FailNow()
}

func (t *t) Errorf(format string, args ...interface{}) {
	t.msgs = append(t.msgs, Msg{Msg: fmt.Sprintf(format, args...), Level: LevelError})
	t.failed = true
}

func (t *t) Helper() {
	// nothing to do here, I think?
}

func (t *t) Log(args ...interface{}) {
	t.msgs = append(t.msgs, Msg{Msg: fmt.Sprintln(args...), Level: LevelInfo})
}

func (t *t) Logf(format string, args ...interface{}) {
	t.msgs = append(t.msgs, Msg{Msg: fmt.Sprintf(format, args...), Level: LevelInfo})
}

func (t *t) Run(name string, tester Tester) bool {
	if !t.test() {
		panic("attempting to run subtest on non-subtest-capable T (you can only Run in Tests, not Before/After)")
	}
	t.subtests <- subtest{
		name:   strings.Map(sanitizeName, name),
		tester: tester,
	}
	return <-t.subtestDone
}

// Parallel does nothing for this implementation.
// TODO figure out how to support it.
func (*t) Parallel() {}

func (t *t) Cleanup(f func()) {
	if t.lifecycle == nil {
		panic("testy: Cleanup is only supported in Test and Run callbacks")
	}
	if f == nil {
		panic("testy: nil cleanup")
	}
	t.lifecycle.add(func() {
		// Record each panic before proceeding to the remaining callbacks, which
		// may themselves call FailNow or Skipf (Goexit).
		defer t.recoverPanic()
		f()
	})
}

func (t *t) Context() context.Context {
	if t.lifecycle == nil {
		panic("testy: Context is only supported in Test and Run callbacks")
	}
	return t.lifecycle.ctx
}

func (t *t) recoverPanic() {
	if err := recover(); err != nil {
		t.Errorf("panic: %+v", err)
	}
}

func (t *t) Skipf(format string, args ...interface{}) {
	if !t.test() {
		panic("testy: Skipf is only supported in Test and Run callbacks")
	}
	t.skipped = true
	t.skipReason = fmt.Sprintf(format, args...)
	t.Logf("skipped: %s", t.skipReason)
	runtime.Goexit()
}

func (t *t) Skipped() bool { return t.skipped }

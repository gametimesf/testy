package testy

import (
	"fmt"
	"runtime"
	"strings"
)

type t struct {
	name        string
	tester      Tester
	failed      bool
	msgs        []Msg
	subtests    chan<- subtest
	subtestDone <-chan struct{}
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

func (t *t) Name() string {
	return t.name
}

func (t *t) Run(name string, tester Tester) {
	if !t.test() {
		panic("attempting to run subtest on non-subtest-capable T (you can only Run in Tests, not Before/After)")
	}
	t.subtests <- subtest{
		name:   strings.Map(stripName, name),
		tester: tester,
	}
	<-t.subtestDone
}

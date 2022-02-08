package testy

import (
	"fmt"
	"runtime"
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
	t.failed = true
	runtime.Goexit()
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
	t.subtests <- subtest{
		name:   name,
		tester: tester,
	}
	<-t.subtestDone
}

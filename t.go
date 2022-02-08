package testy

import (
	"fmt"
	"runtime"
)

type T struct {
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

var _ TestingT = (*T)(nil)

func (t *T) run() {
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

func (t *T) Fail() {
	t.failed = true
}

func (t *T) FailNow() {
	t.failed = true
	runtime.Goexit()
}

func (t *T) Fatal(args ...interface{}) {
	t.msgs = append(t.msgs, Msg{Msg: fmt.Sprintln(args...), Level: LevelError})
	t.FailNow()
}

func (t *T) Fatalf(format string, args ...interface{}) {
	t.msgs = append(t.msgs, Msg{Msg: fmt.Sprintf(format, args...), Level: LevelError})
	t.FailNow()
}

func (t *T) Errorf(format string, args ...interface{}) {
	t.msgs = append(t.msgs, Msg{Msg: fmt.Sprintf(format, args...), Level: LevelError})
	t.failed = true
}

func (t *T) Helper() {
	// nothing to do here, I think?
}

func (t *T) Log(args ...interface{}) {
	t.msgs = append(t.msgs, Msg{Msg: fmt.Sprintln(args...), Level: LevelInfo})
}

func (t *T) Logf(format string, args ...interface{}) {
	t.msgs = append(t.msgs, Msg{Msg: fmt.Sprintf(format, args...), Level: LevelInfo})
}

func (t *T) Name() string {
	return t.name
}

func (t *T) Run(name string, tester Tester) {
	t.subtests <- subtest{
		name:   name,
		tester: tester,
	}
	<-t.subtestDone
}

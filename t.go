package testy

import (
	"fmt"
	"runtime"
)

type T struct {
	name   string
	failed bool
	// TODO differentiate logs from errors
	msgs []string
}

var _ TestingT = (*T)(nil)

func (t *T) Fail() {
	t.failed = true
}

func (t *T) FailNow() {
	t.failed = true
	runtime.Goexit()
}

func (t *T) Fatal(args ...interface{}) {
	t.msgs = append(t.msgs, fmt.Sprintln(args...))
	t.FailNow()
}

func (t *T) Fatalf(format string, args ...interface{}) {
	t.msgs = append(t.msgs, fmt.Sprintf(format, args...))
	t.FailNow()
}

func (t *T) Errorf(format string, args ...interface{}) {
	t.msgs = append(t.msgs, fmt.Sprintf(format, args...))
	t.failed = true
}

func (t *T) Helper() {
	// nothing to do here, I think?
}

func (t *T) Log(args ...interface{}) {
	t.msgs = append(t.msgs, fmt.Sprintln(args...))
}

func (t *T) Logf(format string, args ...interface{}) {
	t.msgs = append(t.msgs, fmt.Sprintf(format, args...))
}

func (t *T) Name() string {
	return t.name
}

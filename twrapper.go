package testy

import (
	"testing"
)

// tWrapper wraps a real testing.T, because Run takes a concrete implementation.
type tWrapper struct {
	t *testing.T
}

var _ TestingT = (*tWrapper)(nil)

func (t tWrapper) Fail() {
	t.t.Fail()
}

func (t tWrapper) FailNow() {
	t.t.FailNow()
}

func (t tWrapper) Fatal(args ...interface{}) {
	t.t.Fatal(args...)
}

func (t tWrapper) Fatalf(format string, args ...interface{}) {
	t.t.Fatalf(format, args...)
}

func (t tWrapper) Errorf(format string, args ...interface{}) {
	t.t.Errorf(format, args...)
}

func (t tWrapper) Helper() {
	t.t.Helper()
}

func (t tWrapper) Log(args ...interface{}) {
	t.t.Log(args...)
}

func (t tWrapper) Logf(format string, args ...interface{}) {
	t.t.Logf(format, args...)
}

func (t tWrapper) Name() string {
	return t.t.Name()
}

func (t tWrapper) Run(s string, tester Tester) {
	t.t.Run(s, func(tt *testing.T) {
		tester(tWrapper{t: tt})
	})
}

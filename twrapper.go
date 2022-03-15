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

func (t tWrapper) Name() string {
	return t.t.Name()
}

func (t tWrapper) Run(s string, tester Tester) bool {
	t.t.Helper()
	return t.t.Run(s, func(tt *testing.T) {
		t.t.Helper()
		tester(tWrapper{t: tt})
	})
}

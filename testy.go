package testy

import (
	"time"

	"github.com/gametimesf/testy/internal/orderedmap"
)

type testy struct {
	tests orderedmap.OrderedMap[string, *testPkg]
	db    DB
}

type testPkg struct {
	name          string
	tests         orderedmap.OrderedMap[string, testCase]
	BeforePackage Tester
	AfterPackage  Tester
	BeforeTest    Tester
	AfterTest     Tester
}

type testCase struct {
	Package string
	Name    string
	tester  Tester
}

// Tester is a thing that runs a test.
type Tester func(t TestingT)

// TestResult is the result of a specific test.
type TestResult struct {
	// Package is the Go package that contains the test.
	Package string
	// Name is the name of the test as provided to Test (for top-level tests), Run (for subtests),
	// or the string representation of each value (for TestEach).
	Name string
	// Msgs contains each message that was emitted during the test via the methods on TestingT that emit messages.
	Msgs []Msg
	// Result is the result of the test.
	Result Result
	// Started is when the test was started.
	Started time.Time
	// Dur is how long the test took.
	Dur time.Duration
	// DurHuman is how long the test took in human-readable form.
	DurHuman string
	// Subtests contains the test result of every test this test started via Run or TestEach.
	Subtests []TestResult
}

// Level indicates at what log level a Msg was emitted.
type Level string

const (
	// LevelInfo is an informative log message (Log, etc.)
	LevelInfo Level = "info"
	// LevelError is an error log message (Fatal, etc.)
	LevelError Level = "error"
)

// Result indicates the result of a test.
type Result string

const (
	// ResultPassed indicates that this test (and all of its subtests) passed.
	ResultPassed Result = "passed"
	// ResultFailed indicates that this test or at least one of its subtests failed.
	ResultFailed Result = "failed"
)

type Msg struct {
	Msg   string
	Level Level
}

var instance testy

// TestingT is a subset of testing.T that we have to implement for non-`go test` runs.
//
// TODO flesh this out with more useful stuff from testing.T -- Parallel would be nice but tricky
type TestingT interface {
	// Fail marks the function as having failed but continues execution.
	Fail()
	// FailNow marks the function as having failed and stops its execution
	// by calling runtime.Goexit (which then runs all deferred calls in the
	// current goroutine).
	// Execution will continue at the next test or benchmark.
	// FailNow must be called from the goroutine running the
	// test or benchmark function, not from other goroutines
	// created during the test. Calling FailNow does not stop
	// those other goroutines.
	FailNow()
	// Fatal is equivalent to Log followed by FailNow.
	Fatal(args ...interface{})
	// Fatalf is equivalent to Logf followed by FailNow.
	Fatalf(format string, args ...interface{})
	// Errorf is equivalent to Logf followed by Fail.
	Errorf(format string, args ...interface{})
	// Helper does not do anything useful since the call stack when passed to the actual implementation has an extra
	// level in it.
	Helper()
	// Log formats its arguments using default formatting, analogous to Println,
	// and records the text in the error log. For tests, the text will be printed only if
	// the test fails or the -test.v flag is set.
	Log(args ...interface{})
	// Logf formats its arguments according to the format, analogous to Printf, and
	// records the text in the error log. A final newline is added if not provided. For
	// tests, the text will be printed only if the test fails or the -test.v flag is
	// set.
	Logf(format string, args ...interface{})
	// Run runs f as a subtest of t called name. It runs f in a separate goroutine
	// and blocks until f returns (or, if running via go test, calls t.Parallel to become a parallel test).
	// Run reports whether f succeeded (or, if running via go test, at least did not fail before calling t.Parallel).
	//
	// Run may be called simultaneously from multiple goroutines, but all such calls
	// must return before the outer test function for t returns.
	Run(string, Tester) bool
	// Parallel signals that this test is to be run in parallel with (and only with)
	// other parallel tests. When a test is run multiple times due to use of
	// -test.count or -test.cpu, multiple instances of a single test never run in
	// parallel with each other.
	//
	// Parallel only affects RunAsTest as it relies on testing.T's implementation.
	Parallel()
}

func sanitizeName(r rune) rune {
	if r == '/' {
		// / is in the middle of the range we do want to allow; we need to strip this since it's used to separate
		// subtests
		return '_'
	} else if r >= '!' && r <= '~' {
		// allow any non-whitespace standard printable character
		return r
	} else {
		// replace anything else
		return '_'
	}
}

// SumTestStats returns the total number of leaf subtests, as well as the number of those that passed and failed.
func (tr TestResult) SumTestStats() (total, passed, failed int) {
	if len(tr.Subtests) == 0 {
		if tr.Result == ResultFailed {
			return 1, 0, 1
		} else {
			return 1, 1, 0
		}
	}

	for _, st := range tr.Subtests {
		t, p, f := st.SumTestStats()
		total += t
		passed += p
		failed += f
	}
	return total, passed, failed
}

// TotalSubtests returns the total number of leaf subtests.
// Prefer to use SumTestStats, as that returns more information for the same recursion cost;
// this is intended for Go templates, which are more limited in what you can do.
func (tr TestResult) TotalSubtests() int {
	total, _, _ := tr.SumTestStats()
	return total
}

// PassedSubtests returns the number of leaf subtests that passed.
// Prefer to use SumTestStats, as that returns more information for the same recursion cost;
// this is intended for Go templates, which are more limited in what you can do.
func (tr TestResult) PassedSubtests() int {
	_, passed, _ := tr.SumTestStats()
	return passed
}

// FailedSubtests returns the number of leaf subtests that failed.
// Prefer to use SumTestStats, as that returns more information for the same recursion cost;
// this is intended for Go templates, which are more limited in what you can do.
func (tr TestResult) FailedSubtests() int {
	_, _, failed := tr.SumTestStats()
	return failed
}

// FindFailingTests finds the least deeply nested subtests that have sibling tests that passed.
// These subtests may be in different branches of subtests.
// This implies that this test failed; if it did not, then a nil slice is returned.
// If every subtest of test failed or if test has no subtests, then test itself is returned.
func (tr TestResult) FindFailingTests() []TestResult {
	if tr.Result != ResultFailed {
		return nil
	}

	total, _, failed := tr.SumTestStats()
	if total == failed {
		return []TestResult{tr}
	}

	// not everything under us has failed, so find what has
	var res []TestResult
	for _, st := range tr.Subtests {
		res = append(res, st.FindFailingTests()...)
	}
	return res
}

// TruncatedTimestamp returns the started timestamp truncated to second precision.
func (tr TestResult) TruncatedTimestamp() time.Time {
	return tr.Started.Truncate(time.Second)
}

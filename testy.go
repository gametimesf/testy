package testy

import (
	"time"

	"github.com/gametimesf/testy/internal/orderedmap"
)

type testy struct {
	tests orderedmap.OrderedMap[string, *testPkg]
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

type Tester func(t TestingT)

type TestResult struct {
	Package  string
	Name     string
	Msgs     []Msg
	Result   Result
	Dur      time.Duration
	DurHuman string
	Subtests []TestResult
}

type Level string

const (
	LevelInfo  Level = "info"
	LevelError Level = "error"
)

type Result string

const (
	ResultPassed Result = "passed"
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
	Fail()
	FailNow()
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Helper()
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Name() string
	Run(string, Tester) bool
}

func stripName(r rune) rune {
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

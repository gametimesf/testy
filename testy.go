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
	Run(string, Tester)
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

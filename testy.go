package testy

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
)

type testy struct {
	// TODO hierarchy? need an ordered map, this is shortly on TODO list and also why 1.18beta in use for generics
	tests []*Test
}

var instance testy

// TestingT is a subset of testing.T that we have to implement for non-`go test` runs.
//
// TODO flesh this out with more useful stuff from testing.T
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
}

type Tester func(t TestingT)

type Test struct {
	Package string
	Name    string
	tester  Tester
	// TODO better results
	Msgs   []string
	Failed bool
}

func RegisterTest(name string, tester Tester) interface{} {
	// we only care about our immediate caller
	callers := make([]uintptr, 1)
	// skip over Callers and ourselves
	n := runtime.Callers(2, callers[:])
	pkg := "<unknown>"
	// make sure it was able to actually get our caller
	if n > 0 {
		frames := runtime.CallersFrames(callers)
		frame, _ := frames.Next()
		// remove the function name (which is almost certainly "init") and leave just the package name.
		// as an example, this function is `github.com/gametimesf/testy.RegisterTest`.
		i := strings.LastIndex(frame.Function, ".")
		pkg = frame.Function[:i]
	}

	instance.tests = append(instance.tests, &Test{
		Package: pkg,
		Name:    name,
		tester:  tester,
	})

	return nil
}

// RunAsTest runs all registered tests under Go's testing framework. To run tests on a per-package basis, put a test
// file in each package containing a single test that calls this function. This is recommended so accurate per-package
// execution times are reported, as well as using the test cache. Do not import a test package into another test package
// as that will cause the tests in the second package to get executed with the first package. If code or resources need
// shared between test packages, put them in their own package which does not contain any test definitions.
func RunAsTest(t *testing.T) {
	t.Helper()
	for _, test := range instance.tests {
		t.Run(test.Name, func(tt *testing.T) {
			tt.Helper()
			test.tester(tt)
		})
	}
}

// Run runs all registered tests and returns result information about them.
//
// TODO: ability to filter for specific packages and tests
// TODO: channel for results to support progressive progress loading? or some sort of ID and background processing?
func Run() []*Test {
	for _, test := range instance.tests {
		t := &T{name: test.Name}
		wg := sync.WaitGroup{}
		wg.Add(1)
		// run in another goroutine so FailNow can work
		go func() {
			defer func() {
				// catch panics and mark test as failed
				if err := recover(); err != nil {
					t.msgs = append(t.msgs, fmt.Sprintf("panic: %+v", err))
					t.failed = true
				}

				wg.Done()
			}()
			test.tester(t)
		}()
		wg.Wait()
		test.Msgs = t.msgs
		test.Failed = t.failed
	}

	return instance.tests
}

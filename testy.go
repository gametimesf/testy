package testy

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gametimesf/testy/orderedmap"
)

type testy struct {
	tests orderedmap.OrderedMap[string, orderedmap.OrderedMap[string, testCase]]
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

type Tester func(t TestingT)

type testCase struct {
	Package string
	Name    string
	tester  Tester
}

type Level string

const (
	LevelInfo  Level = "info"
	LevelError Level = "error"
)

type Msg struct {
	Msg   string
	Level Level
}

type TestResult struct {
	Package string
	Name    string
	Msgs    []Msg
	Passed  bool
	Dur     time.Duration
}

var regLock sync.Mutex

func RegisterTest(name string, tester Tester) any {
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

	regLock.Lock()
	defer regLock.Unlock()
	if instance.tests == nil {
		instance.tests = make(orderedmap.OrderedMap[string, orderedmap.OrderedMap[string, testCase]])
	}
	if instance.tests[pkg] == nil {
		instance.tests[pkg] = make(orderedmap.OrderedMap[string, testCase])
	}

	if _, exists := instance.tests[pkg][name]; exists {
		panic(fmt.Sprintf("test %s already exists in package %s", name, pkg))
	}

	instance.tests[pkg][name] = testCase{
		Package: pkg,
		Name:    name,
		tester:  tester,
	}

	return nil
}

// RunAsTest runs all registered tests under Go's testing framework. To run tests on a per-package basis, put a test
// file in each package containing a single test that calls this function. This is recommended so accurate per-package
// execution times are reported, as well as using the test cache. Do not import a test package into another test package
// as that will cause the tests in the second package to get executed with the first package. If code or resources need
// shared between test packages, put them in their own package which does not contain any test definitions.
func RunAsTest(t *testing.T) {
	t.Helper()
	instance.tests.Iterate(func(pkg string, tests orderedmap.OrderedMap[string, testCase]) bool {
		// for _, tests := range instance.tests {
		tests.Iterate(func(name string, test testCase) bool {
			// for _, test := range tests {
			t.Run(test.Name, func(tt *testing.T) {
				tt.Helper()
				test.tester(tWrapper{t: tt})
			})
			return true
		})
		return true
	})
}

// Run runs all registered tests and returns result information about them.
//
// TODO: ability to filter for specific packages and tests
// TODO: channel for results to support progressive result loading?
func Run() orderedmap.OrderedMap[string, orderedmap.OrderedMap[string, TestResult]] {
	results := make(orderedmap.OrderedMap[string, orderedmap.OrderedMap[string, TestResult]])

	instance.tests.Iterate(func(pkg string, tests orderedmap.OrderedMap[string, testCase]) bool {
		results[pkg] = make(orderedmap.OrderedMap[string, TestResult])
		tests.Iterate(func(name string, test testCase) bool {

			res := make(chan TestResult)

			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				for r := range res {
					results[pkg][r.Name] = r
				}
			}()

			runTest(pkg, test.Name, test.tester, res)
			close(res)
			wg.Wait()

			return true
		})
		return true
	})

	return results
}

func runTest(pkg, baseName string, tester Tester, results chan<- TestResult) {
	subtests := make(chan subtest)
	subtestDone := make(chan struct{})
	t := &T{
		name:        baseName,
		tester:      tester,
		subtests:    subtests,
		subtestDone: subtestDone,
	}

	stWg := sync.WaitGroup{}
	stWg.Add(1)

	go func() {
		defer stWg.Done()
		for st := range subtests {
			runTest(pkg, baseName+"/"+st.name, st.tester, results)
			subtestDone <- struct{}{}
		}
	}()

	wg := sync.WaitGroup{}
	wg.Add(1)

	start := time.Now()
	// run in another goroutine so FailNow can work
	go func() {
		defer wg.Done()
		t.run()
	}()
	// wait for original test to finish
	wg.Wait()
	close(subtests)
	stWg.Wait()
	close(subtestDone)
	dur := time.Now().Sub(start)

	// TODO handle t.Run calls and recurse

	results <- TestResult{
		Package: pkg,
		Name:    baseName,
		Msgs:    t.msgs,
		Passed:  !t.failed,
		Dur:     dur,
	}
}

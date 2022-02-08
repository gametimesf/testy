package testy

import (
	"sync"
	"testing"
	"time"

	"github.com/gametimesf/testy/orderedmap"
)

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
	t := &t{
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
	// this shouldn't be needed since the test actually waits for the subtest to complete before continuing, but it
	// doesn't hurt to be careful
	stWg.Wait()
	close(subtestDone)
	dur := time.Now().Sub(start)

	results <- TestResult{
		Package: pkg,
		Name:    baseName,
		Msgs:    t.msgs,
		Passed:  !t.failed,
		Dur:     dur,
	}
}

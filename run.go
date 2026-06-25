package testy

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"testing"
	"time"
)

// ErrPackageNotFound indicates a requested registered package does not exist.
var ErrPackageNotFound = errors.New("package not found")

// RunOptions controls how the non-go-test runner executes registered tests.
type RunOptions struct {
	// PackageConcurrency is the maximum number of packages executed at once.
	// Values <= 1 preserve the historical serial package execution behavior.
	PackageConcurrency int
}

// PackageSpec describes a package registered with testy.
type PackageSpec struct {
	// Name is the package import path.
	Name string
	// Index is the package's deterministic registration-order index.
	Index int
}

// RunAsTest runs all registered tests under Go's testing framework.
//
// To run tests on a per-package basis, put a test file in each package containing a single test that calls this function.
// This is recommended so accurate per-package execution times are reported, as well as using the test cache.
// Do not import a test package into another test package as that will cause the tests in the second package to get executed with the first package.
// If code or resources need shared between test packages, put them in their own package which does not contain any test definitions.
//
// Individual tests in a package may still be run using the standard -run test flag.
// See `go help testflag` for more information.
//
// TODO: shuffle test execution order (see -shuffle in `go help testflag`)
func RunAsTest(t *testing.T) {
	t.Helper()
	instance.tests.Iterate(func(pkg string, pkgTests *testPkg) bool {
		// we have to hold onto any panics here to be able to run AfterPackage
		var beforePkgErr any
		if pkgTests.BeforePackage != nil {
			func() {
				defer func() {
					if beforePkgErr = recover(); beforePkgErr != nil {
						beforePkgErr = fmt.Sprintf("before package: %v\n\n%s", beforePkgErr, debug.Stack())
					}
				}()
				pkgTests.BeforePackage(tWrapper{t: t})
			}()
		}

		// only run the tests if any BeforePackage didn't panic
		if beforePkgErr == nil {
			pkgTests.tests.Iterate(func(name string, test testCase) bool {
				t.Run(test.Name, func(tt *testing.T) {
					tt.Helper()

					// if we have an AfterTest, defer it so it always runs even if BeforeTest or the test itself panic
					if pkgTests.AfterTest != nil {
						defer pkgTests.AfterTest(tWrapper{t: tt})
					}

					// if we have a BeforeTest, just run it directly; panics will sort themselves out
					if pkgTests.BeforeTest != nil {
						pkgTests.BeforeTest(tWrapper{t: tt})
					}

					test.tester(tWrapper{t: tt})
				})
				return true
			})
		}

		// and we have to hold onto any panics here to give priority to any panics from BeforePackage
		var afterPkgErr any
		if pkgTests.AfterPackage != nil {
			func() {
				defer func() {
					if afterPkgErr = recover(); afterPkgErr != nil {
						afterPkgErr = fmt.Sprintf("after package: %v\n\n%s", afterPkgErr, debug.Stack())
					}
				}()
				pkgTests.AfterPackage(tWrapper{t: t})
			}()
		}

		if beforePkgErr != nil {
			panic(beforePkgErr)
		}
		if afterPkgErr != nil {
			panic(afterPkgErr)
		}

		return true
	})
}

// Run runs all registered tests and returns result information about them.
//
// TODO: ability to filter for specific packages and tests
//
// TODO: shuffle test execution order (see -shuffle in `go help testflag`)
//
// TODO: channel for results to support progressive result loading?
func Run() TestResult {
	return RunWithOptions(RunOptions{PackageConcurrency: 1})
}

// RunWithOptions runs all registered tests and returns result information about them.
func RunWithOptions(opts RunOptions) TestResult {
	start := time.Now()

	pkgs := collectPackages()
	packageResults := make([]TestResult, len(pkgs))

	packageConcurrency := opts.PackageConcurrency
	if packageConcurrency < 1 {
		packageConcurrency = 1
	}

	if packageConcurrency == 1 {
		for i := range pkgs {
			packageResults[i] = runPackage(pkgs[i].name, pkgs[i].tests)
		}
	} else {
		runPackagesConcurrently(pkgs, packageResults, packageConcurrency)
	}

	return BuildSuiteResult(start, packageResults)
}

// ListPackages returns the registered packages in deterministic execution order.
//
// The returned package list is intentionally independent from any execution
// backend. Callers that want to orchestrate package execution outside this
// process, such as a durable workflow engine, can use the package names with
// RunPackage and then combine the package results with BuildSuiteResult.
func ListPackages() []PackageSpec {
	pkgs := collectPackages()
	specs := make([]PackageSpec, len(pkgs))
	for i, pkg := range pkgs {
		specs[i] = PackageSpec{
			Name:  pkg.name,
			Index: i,
		}
	}
	return specs
}

// RunPackage runs a single registered package by import path.
func RunPackage(pkg string) (TestResult, error) {
	pkgTests, ok := instance.tests[pkg]
	if !ok {
		return TestResult{}, fmt.Errorf("%w: %s", ErrPackageNotFound, pkg)
	}
	return runPackage(pkg, pkgTests), nil
}

// BuildSuiteResult combines package-level results into a suite-level result.
func BuildSuiteResult(start time.Time, packageResults []TestResult) TestResult {
	results := TestResult{
		Name:     "Test Suite",
		Started:  start,
		Subtests: packageResults,
	}

	r := ResultPassed
	for _, pkgResult := range packageResults {
		if pkgResult.Result == ResultFailed {
			r = ResultFailed
			break
		}
	}
	results.Result = r
	dur := time.Since(start).Round(time.Millisecond)
	results.Dur = dur
	results.DurHuman = dur.String()
	return results
}

func runPackagesConcurrently(pkgs []runPackageInput, results []TestResult, packageConcurrency int) {
	sem := make(chan struct{}, packageConcurrency)
	wg := sync.WaitGroup{}
	for i := range pkgs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = runPackage(pkgs[i].name, pkgs[i].tests)
		}(i)
	}
	wg.Wait()
}

type runPackageInput struct {
	name  string
	tests *testPkg
}

func collectPackages() []runPackageInput {
	var pkgs []runPackageInput
	instance.tests.Iterate(func(pkg string, pkgTests *testPkg) bool {
		pkgs = append(pkgs, runPackageInput{name: pkg, tests: pkgTests})
		return true
	})
	return pkgs
}

func runPackage(pkg string, pkgTests *testPkg) TestResult {
	pkgStart := time.Now()
	pkgResults := TestResult{
		Package: pkg,
		Name:    "Package",
		Started: pkgStart,
	}

	pkgHelperT := &t{}
	pkgAnyFailures := false

	// we have to hold onto any panics here to be able to run AfterPackage
	var beforePkgErr any
	if pkgTests.BeforePackage != nil {
		func() {
			defer func() {
				if beforePkgErr = recover(); beforePkgErr != nil {
					beforePkgErr = fmt.Sprintf("before package: %v\n\n%s", beforePkgErr, debug.Stack())
				}
			}()
			pkgTests.BeforePackage(pkgHelperT)
		}()

		if beforePkgErr != nil {
			pkgAnyFailures = true
			pkgResults.Msgs = []Msg{
				{
					Msg:   fmt.Sprintf("%v", beforePkgErr),
					Level: LevelError,
				},
			}
		}
	}

	// we still have to iterate even if there was a BeforePackage panic to be able to fail all the tests
	pkgTests.tests.Iterate(func(name string, test testCase) bool {
		// only run the tests if BeforePackage didn't panic
		if beforePkgErr == nil {
			testHelperT := &t{}

			// we have to hold onto any panics here to be able to run AfterTest
			var beforeTestErr any
			if pkgTests.BeforeTest != nil {
				func() {
					defer func() {
						if beforeTestErr = recover(); beforeTestErr != nil {
							beforeTestErr = fmt.Sprintf("before test: %v\n\n%s", beforeTestErr, debug.Stack())
						}
					}()
					pkgTests.BeforeTest(testHelperT)
				}()
			}

			// only run the tests if any BeforeTest didn't panic
			if beforeTestErr == nil {
				res := runTest(pkg, test.Name, test.tester)
				if res.Result == ResultFailed {
					pkgAnyFailures = true
				}
				pkgResults.Subtests = append(pkgResults.Subtests, res)
			} else {
				pkgAnyFailures = true
				pkgResults.Subtests = append(pkgResults.Subtests, TestResult{
					Package:  pkg,
					Name:     name,
					Started:  time.Now(),
					Result:   ResultFailed,
					Dur:      0,
					DurHuman: "0s",
					Msgs: append(testHelperT.msgs, Msg{
						Msg:   fmt.Sprintf("%v", beforeTestErr),
						Level: LevelError,
					}),
				})
			}

			if pkgTests.AfterTest != nil {
				var afterTestErr any
				func() {
					defer func() {
						if afterTestErr = recover(); afterTestErr != nil {
							afterTestErr = fmt.Sprintf("after test: %v\n\n%s", afterTestErr, debug.Stack())
						}
					}()
					pkgTests.AfterTest(testHelperT)
				}()

				if afterTestErr != nil {
					pkgAnyFailures = true
					// update test results marking it failed and with this panic message.
					r := &pkgResults.Subtests[len(pkgResults.Subtests)-1]
					r.Result = ResultFailed
					r.Msgs = append(r.Msgs, append(testHelperT.msgs, Msg{
						Msg:   fmt.Sprintf("%v", afterTestErr),
						Level: LevelError,
					})...)
				}
			}
		} else {
			pkgAnyFailures = true
			// BeforePackage panicked, so simply mark the test as failed with its message
			pkgResults.Subtests = append(pkgResults.Subtests, TestResult{
				Package:  pkg,
				Name:     name,
				Started:  pkgStart,
				Result:   ResultFailed,
				Dur:      0,
				DurHuman: "0s",
				Msgs: append(pkgHelperT.msgs, Msg{
					Msg:   fmt.Sprintf("%v", beforePkgErr),
					Level: LevelError,
				}),
			})
		}

		return true
	})

	var afterPkgErr any
	if pkgTests.AfterPackage != nil {
		func() {
			defer func() {
				if afterPkgErr = recover(); afterPkgErr != nil {
					afterPkgErr = fmt.Sprintf("after package: %v\n\n%s", afterPkgErr, debug.Stack())
				}
			}()
			pkgTests.AfterPackage(pkgHelperT)
		}()
	}

	// update test results if AfterPackage panicked
	if afterPkgErr != nil {
		pkgAnyFailures = true
		m := Msg{
			Msg:   fmt.Sprintf("%v", afterPkgErr),
			Level: LevelError,
		}
		for i := range pkgResults.Subtests {
			r := &pkgResults.Subtests[i]
			r.Result = ResultFailed
			r.Msgs = append(r.Msgs, append(pkgHelperT.msgs, m)...)
		}
		pkgResults.Msgs = append(pkgResults.Msgs, m)
	}

	r := ResultPassed
	if pkgAnyFailures {
		r = ResultFailed
	}
	pkgResults.Result = r
	dur := time.Since(pkgStart).Round(time.Millisecond)
	pkgResults.Dur = dur
	pkgResults.DurHuman = dur.String()

	return pkgResults
}

func runTest(pkg, baseName string, tester Tester) TestResult {
	result := TestResult{
		Package: pkg,
		Name:    baseName,
	}

	subtests := make(chan subtest)
	subtestDone := make(chan bool)
	t := &t{
		name:        baseName,
		tester:      tester,
		subtests:    subtests,
		subtestDone: subtestDone,
	}

	stWg := sync.WaitGroup{}
	stWg.Add(1)

	anyFailures := false
	go func() {
		defer stWg.Done()
		for st := range subtests {
			stResult := runTest(pkg, baseName+"/"+st.name, st.tester)
			if stResult.Result == ResultFailed {
				// TODO does this need to be an atomic operation?
				anyFailures = true
			}
			result.Subtests = append(result.Subtests, stResult)
			subtestDone <- stResult.Result != ResultFailed
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
	dur := time.Since(start).Round(time.Millisecond)

	r := ResultPassed
	if t.failed || anyFailures {
		r = ResultFailed
	}
	result.Msgs = t.msgs
	result.Result = r
	result.Started = start
	result.Dur = dur
	result.DurHuman = dur.String()
	return result
}

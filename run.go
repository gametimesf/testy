package testy

import (
	"context"
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
	// CaseExecutor is shared across package activities in one worker. Nil keeps
	// case execution serial and preserves existing package concurrency behavior.
	CaseExecutor *CaseExecutor
	// CaseOrder is an optional immutable complete permutation of case names per
	// package. It changes admission order only, never result identity or steps.
	// Missing/invalid hints retain canonical order and invalid hints are reported.
	CaseOrder map[string][]string
}

// PackageSpec describes a package registered with testy.
type PackageSpec struct {
	// Name is the package import path.
	Name string
	// Index is the package's deterministic registration-order index.
	Index int
}

// CaseSpec describes canonical registered case identity, not a nested step.
type CaseSpec struct {
	Name       string
	Index      int
	Concurrent bool
}

// ListCases returns a snapshot in canonical result order. Scheduling callers may
// use it to validate history without introspecting test callbacks or executing them.
func ListCases(pkg string) ([]CaseSpec, error) {
	tests, ok := instance.tests[pkg]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrPackageNotFound, pkg)
	}
	var cases []CaseSpec
	tests.tests.Iterate(func(name string, test testCase) bool {
		cases = append(cases, CaseSpec{Name: name, Index: len(cases), Concurrent: test.Concurrent})
		return true
	})
	return cases, nil
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
		t.Cleanup(func() {
			var afterPkgErr any
			if pkgTests.AfterPackage != nil {
				func() {
					defer func() { afterPkgErr = recover() }()
					pkgTests.AfterPackage(tWrapper{t: t})
				}()
			}
			if beforePkgErr != nil {
				panic(beforePkgErr)
			}
			if afterPkgErr != nil {
				panic(fmt.Sprintf("after package: %v", afterPkgErr))
			}
		})

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
					if test.Concurrent {
						tt.Parallel()
					}

					// Register enclosing teardown before the test lifecycle so it runs
					// after resource cleanup, including fatal/panic and parallel children.
					if pkgTests.AfterTest != nil {
						tt.Cleanup(func() { pkgTests.AfterTest(tWrapper{t: tt}) })
					}

					// if we have a BeforeTest, just run it directly; panics will sort themselves out
					if pkgTests.BeforeTest != nil {
						pkgTests.BeforeTest(tWrapper{t: tt})
					}

					newTWrapper(tt, context.Background()).run(test.tester)
				})
				return true
			})
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
	return RunWithContext(context.Background(), opts)
}

// RunWithContext cancels case contexts and drains their lifecycle before returning.
// Callbacks must cooperate with cancellation; arbitrary Go code is not preempted.
func RunWithContext(ctx context.Context, opts RunOptions) TestResult {
	start := time.Now()

	pkgs := collectPackages()
	packageResults := make([]TestResult, len(pkgs))

	packageConcurrency := opts.PackageConcurrency
	if packageConcurrency < 1 {
		packageConcurrency = 1
	}

	if packageConcurrency == 1 {
		for i := range pkgs {
			packageResults[i] = runPackageWithOptions(ctx, pkgs[i].name, pkgs[i].tests, opts)
		}
	} else {
		runPackagesConcurrently(ctx, pkgs, packageResults, packageConcurrency, opts)
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
	return RunPackageWithOptions(context.Background(), pkg, RunOptions{})
}

// RunPackageWithOptions shares the worker executor and propagates cancellation.
// Even on cancellation it waits for admitted callbacks, cleanup and hooks before
// returning a result and ctx.Err; orchestration must not start a replacement early.
func RunPackageWithOptions(ctx context.Context, pkg string, opts RunOptions) (TestResult, error) {
	pkgTests, ok := instance.tests[pkg]
	if !ok {
		return TestResult{}, fmt.Errorf("%w: %s", ErrPackageNotFound, pkg)
	}
	return runPackageWithOptions(ctx, pkg, pkgTests, opts), ctx.Err()
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
		r = combineResults(r, pkgResult.Result)
	}
	results.Result = r
	dur := time.Since(start).Round(time.Millisecond)
	results.Dur = dur
	results.DurHuman = dur.String()
	return results
}

func runPackagesConcurrently(ctx context.Context, pkgs []runPackageInput, results []TestResult, packageConcurrency int, opts RunOptions) {
	sem := make(chan struct{}, packageConcurrency)
	wg := sync.WaitGroup{}
	for i := range pkgs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = runPackageWithOptions(ctx, pkgs[i].name, pkgs[i].tests, opts)
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
	return runPackageWithOptions(context.Background(), pkg, pkgTests, RunOptions{})
}

func runPackageWithOptions(ctx context.Context, pkg string, pkgTests *testPkg, opts RunOptions) TestResult {
	start := time.Now()
	result := TestResult{Package: pkg, Name: "Package", Started: start}
	helper := &t{}
	beforeErr := runPackageHook(ctx, opts.CaseExecutor, "before package", pkgTests.BeforePackage, helper)
	if beforeErr != nil {
		result.Msgs = append(result.Msgs, Msg{Msg: beforeErr.Error(), Level: LevelError})
	}
	var cases []testCase
	pkgTests.tests.Iterate(func(_ string, test testCase) bool { cases = append(cases, test); return true })
	result.Subtests = make([]TestResult, len(cases))
	order, valid := caseDispatchOrder(cases, opts.CaseOrder[pkg])
	if !valid {
		result.Msgs = append(result.Msgs, Msg{Level: LevelInfo, Msg: "case scheduling hint ignored: not a complete permutation of current cases"})
	}
	var wg sync.WaitGroup
	queuedAt := time.Now()
	for _, i := range order {
		if beforeErr != nil {
			result.Subtests[i] = failedCase(pkg, cases[i].Name, start, helper.msgs, beforeErr)
			continue
		}
		release, err := opts.CaseExecutor.acquire(ctx, cases[i].Concurrent)
		if err != nil {
			result.Subtests[i] = failedCase(pkg, cases[i].Name, time.Now(), nil, err)
			continue
		}
		wait := time.Since(queuedAt)
		run := func(i int, release func(), wait time.Duration) {
			defer release()
			result.Subtests[i] = runCaseWithHooks(ctx, pkg, cases[i], pkgTests)
			result.Subtests[i].QueueDur = wait
		}
		if opts.CaseExecutor == nil {
			run(i, release, wait)
			continue
		}
		// Admission follows the supplied snapshot; results stay in canonical order.
		// Only admitted cases own goroutines.
		// Exclusive admission waits for existing lifecycle owners to drain.
		wg.Add(1)
		go func(i int, release func(), wait time.Duration) { defer wg.Done(); run(i, release, wait) }(i, release, wait)
	}
	wg.Wait()
	// Teardown must still run after cancellation, using a fresh admission context.
	afterErr := runPackageHook(context.Background(), opts.CaseExecutor, "after package", pkgTests.AfterPackage, helper)
	if afterErr != nil {
		message := Msg{Msg: afterErr.Error(), Level: LevelError}
		result.Msgs = append(result.Msgs, message)
		for i := range result.Subtests {
			child := &result.Subtests[i]
			child.Result = ResultFailed
			child.Msgs = append(child.Msgs, append(helper.msgs, message)...)
		}
	}
	result.Result = ResultPassed
	for _, child := range result.Subtests {
		result.Result = combineResults(result.Result, child.Result)
	}
	if helper.failed {
		result.Msgs = append(result.Msgs, helper.msgs...)
	}
	if beforeErr != nil || afterErr != nil || helper.failed || ctx.Err() != nil {
		result.Result = ResultFailed
	}
	result.Dur = time.Since(start).Round(time.Millisecond)
	result.DurHuman = result.Dur.String()
	return result
}

func runPackageHook(ctx context.Context, executor *CaseExecutor, label string, hook Tester, helper *t) error {
	if hook == nil {
		return nil
	}
	release, err := executor.acquire(ctx, false)
	if err != nil {
		return err
	}
	defer release()
	return invokeHook(label, hook, helper)
}

func invokeHook(label string, hook Tester, helper *t) (err error) {
	if hook == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%s: %v\n\n%s", label, recovered, debug.Stack())
		}
	}()
	hook(helper)
	return nil
}

func failedCase(pkg, name string, start time.Time, messages []Msg, err error) TestResult {
	return TestResult{Package: pkg, Name: name, Started: start, Result: ResultFailed, DurHuman: "0s", Msgs: append(append([]Msg(nil), messages...), Msg{Msg: err.Error(), Level: LevelError})}
}

func runCaseWithHooks(ctx context.Context, pkg string, test testCase, hooks *testPkg) TestResult {
	start := time.Now()
	if err := ctx.Err(); err != nil {
		return failedCase(pkg, test.Name, start, nil, err)
	}
	helper := &t{}
	beforeErr := invokeHook("before test", hooks.BeforeTest, helper)
	var result TestResult
	if beforeErr != nil {
		result = failedCase(pkg, test.Name, start, helper.msgs, beforeErr)
	} else {
		result = runTestContext(ctx, pkg, test.Name, test.tester)
	}
	if afterErr := invokeHook("after test", hooks.AfterTest, helper); afterErr != nil {
		result.Result = ResultFailed
		result.Msgs = append(result.Msgs, append(helper.msgs, Msg{Msg: afterErr.Error(), Level: LevelError})...)
	}
	if helper.failed {
		result.Result = ResultFailed
		result.Msgs = append(result.Msgs, helper.msgs...)
	}
	if err := ctx.Err(); err != nil {
		result.Result = ResultFailed
		result.Msgs = append(result.Msgs, Msg{Msg: "case canceled: " + err.Error(), Level: LevelError})
	}
	result.Started = start
	result.Dur = time.Since(start).Round(time.Millisecond)
	result.DurHuman = result.Dur.String()
	return result
}

func runTest(pkg, baseName string, tester Tester) TestResult {
	return runTestContext(context.Background(), pkg, baseName, tester)
}

func runTestContext(ctx context.Context, pkg, baseName string, tester Tester) TestResult {
	result := TestResult{
		Package: pkg,
		Name:    baseName,
	}

	subtests := make(chan subtest)
	subtestDone := make(chan bool)
	t := &t{
		lifecycle:   newLifecycle(ctx),
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
			stResult := runTestContext(t.Context(), pkg, baseName+"/"+st.name, st.tester)
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
	for _, child := range result.Subtests {
		r = combineResults(r, child.Result)
	}
	if t.skipped {
		r = ResultSkipped
		if len(result.Subtests) > 0 {
			r = ResultIncomplete
		}
	}
	if t.failed || anyFailures {
		r = ResultFailed
	}
	result.SkipReason = t.skipReason
	result.Msgs = t.msgs
	result.Result = r
	result.Started = start
	result.Dur = dur
	result.DurHuman = dur.String()
	return result
}

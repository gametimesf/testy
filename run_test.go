package testy

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gametimesf/testy/internal/orderedmap"
)

// know that before/after package/test and the test itself have run and when they were run
var bp, bt, at, ap, tt time.Time
var subtestResult *bool

func succeeds(ts *time.Time) Tester {
	return func(TestingT) {
		*ts = time.Now()
	}
}

func TestPackageRunnerAPI(t *testing.T) {
	instance = testy{
		tests: orderedmap.OrderedMap[string, *testPkg]{},
	}
	defer func() {
		instance = testy{}
	}()

	for i := 0; i < 2; i++ {
		pkg := fmt.Sprintf("github.com/gametimesf/testy/packageapi/pkg%d", i)
		testName := fmt.Sprintf("test%d", i)
		instance.tests[pkg] = &testPkg{
			name:  pkg,
			tests: orderedmap.OrderedMap[string, testCase]{},
		}
		instance.tests[pkg].tests[testName] = testCase{
			Package: pkg,
			Name:    testName,
			tester:  succeeds(&tt),
		}
	}

	pkgs := ListPackages()
	require.Equal(t, []PackageSpec{
		{Name: "github.com/gametimesf/testy/packageapi/pkg0", Index: 0},
		{Name: "github.com/gametimesf/testy/packageapi/pkg1", Index: 1},
	}, pkgs)

	start := time.Now()
	packageResults := make([]TestResult, len(pkgs))
	for _, pkg := range pkgs {
		res, err := RunPackage(pkg.Name)
		require.NoError(t, err)
		packageResults[pkg.Index] = res
	}
	suite := BuildSuiteResult(start, packageResults)

	assert.Equal(t, ResultPassed, suite.Result)
	require.Len(t, suite.Subtests, 2)
	assert.Equal(t, "github.com/gametimesf/testy/packageapi/pkg0", suite.Subtests[0].Package)
	assert.Equal(t, "github.com/gametimesf/testy/packageapi/pkg1", suite.Subtests[1].Package)
}

func TestRunPackageReturnsErrPackageNotFound(t *testing.T) {
	instance = testy{}
	defer func() {
		instance = testy{}
	}()

	_, err := RunPackage("github.com/gametimesf/testy/missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPackageNotFound))
}
func panics(ts *time.Time) Tester {
	return func(TestingT) {
		*ts = time.Now()
		panic("panic")
	}
}

func fails(ts *time.Time) Tester {
	return func(t TestingT) {
		*ts = time.Now()
		t.Fatal("fails")
	}
}

func subtestForTest(tester Tester) Tester {
	return func(t TestingT) {
		b := t.Run("subtest", tester)
		subtestResult = &b
	}
}

type runTC struct {
	name          string
	beforePackage Tester
	beforeTest    Tester
	afterTest     Tester
	afterPackage  Tester
	test          Tester
	rootResult    Result
	validate      func(*testing.T, TestResult)
}

var runTCs = []runTC{
	{
		name:          "no helpers, test passes",
		beforePackage: nil,
		beforeTest:    nil,
		afterTest:     nil,
		afterPackage:  nil,
		test:          succeeds(&tt),
		rootResult:    ResultPassed,
		validate: func(t *testing.T, tr TestResult) {
			assert.Zero(t, bp)
			assert.Zero(t, bt)
			assert.NotZero(t, tt)
			assert.Zero(t, at)
			assert.Zero(t, ap)

			assert.Equal(t, ResultPassed, tr.Result)
			assert.Len(t, tr.Msgs, 0)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "no helpers, subtest passes",
		beforePackage: nil,
		beforeTest:    nil,
		afterTest:     nil,
		afterPackage:  nil,
		test:          subtestForTest(succeeds(&tt)),
		rootResult:    ResultPassed,
		validate: func(t *testing.T, tr TestResult) {
			assert.Zero(t, bp)
			assert.Zero(t, bt)
			assert.NotZero(t, tt)
			assert.Zero(t, at)
			assert.Zero(t, ap)

			assert.Equal(t, ResultPassed, tr.Result)

			require.Len(t, tr.Subtests, 1)
			assert.Equal(t, ResultPassed, tr.Subtests[0].Result)
			assert.Len(t, tr.Subtests[0].Msgs, 0)

			require.NotNil(t, subtestResult)
			assert.True(t, *subtestResult)
		},
	},
	{
		name:          "no helpers, test fails",
		beforePackage: nil,
		beforeTest:    nil,
		afterTest:     nil,
		afterPackage:  nil,
		test:          fails(&tt),
		rootResult:    ResultFailed,
		validate: func(t *testing.T, tr TestResult) {
			assert.Zero(t, bp)
			assert.Zero(t, bt)
			assert.NotZero(t, tt)
			assert.Zero(t, at)
			assert.Zero(t, ap)

			assert.Equal(t, ResultFailed, tr.Result)
			assert.Len(t, tr.Msgs, 1)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "no helpers, subtest fails",
		beforePackage: nil,
		beforeTest:    nil,
		afterTest:     nil,
		afterPackage:  nil,
		test:          subtestForTest(fails(&tt)),
		rootResult:    ResultFailed,
		validate: func(t *testing.T, tr TestResult) {
			assert.Zero(t, bp)
			assert.Zero(t, bt)
			assert.NotZero(t, tt)
			assert.Zero(t, at)
			assert.Zero(t, ap)

			assert.Equal(t, ResultFailed, tr.Result)

			require.Len(t, tr.Subtests, 1)
			assert.Equal(t, ResultFailed, tr.Subtests[0].Result)
			assert.Len(t, tr.Subtests[0].Msgs, 1)

			require.NotNil(t, subtestResult)
			assert.False(t, *subtestResult)
		},
	},
	{
		name:          "no helpers, test panics",
		beforePackage: nil,
		beforeTest:    nil,
		afterTest:     nil,
		afterPackage:  nil,
		test:          panics(&tt),
		rootResult:    ResultFailed,
		validate: func(t *testing.T, tr TestResult) {
			assert.Zero(t, bp)
			assert.Zero(t, bt)
			assert.NotZero(t, tt)
			assert.Zero(t, at)
			assert.Zero(t, ap)

			assert.Equal(t, ResultFailed, tr.Result)
			assert.Len(t, tr.Msgs, 1)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "no helpers, subtest panics",
		beforePackage: nil,
		beforeTest:    nil,
		afterTest:     nil,
		afterPackage:  nil,
		test:          subtestForTest(panics(&tt)),
		rootResult:    ResultFailed,
		validate: func(t *testing.T, tr TestResult) {
			assert.Zero(t, bp)
			assert.Zero(t, bt)
			assert.NotZero(t, tt)
			assert.Zero(t, at)
			assert.Zero(t, ap)

			assert.Equal(t, ResultFailed, tr.Result)

			require.Len(t, tr.Subtests, 1)
			assert.Equal(t, ResultFailed, tr.Subtests[0].Result)
			assert.Len(t, tr.Subtests[0].Msgs, 1)

			require.NotNil(t, subtestResult)
			assert.False(t, *subtestResult)
		},
	},
	{
		name:          "all helpers succeed, test passes",
		beforePackage: succeeds(&bp),
		beforeTest:    succeeds(&bt),
		afterTest:     succeeds(&at),
		afterPackage:  succeeds(&ap),
		test:          succeeds(&tt),
		rootResult:    ResultPassed,
		validate: func(t *testing.T, tr TestResult) {
			assert.NotZero(t, bp)
			assert.True(t, bt.After(bp))
			assert.True(t, tt.After(bt))
			assert.True(t, at.After(tt))
			assert.True(t, ap.After(at))

			assert.Equal(t, ResultPassed, tr.Result)
			assert.Len(t, tr.Msgs, 0)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "all helpers succeed, test fails",
		beforePackage: succeeds(&bp),
		beforeTest:    succeeds(&bt),
		afterTest:     succeeds(&at),
		afterPackage:  succeeds(&ap),
		test:          fails(&tt),
		rootResult:    ResultFailed,
		validate: func(t *testing.T, tr TestResult) {
			assert.NotZero(t, bp)
			assert.True(t, bt.After(bp))
			assert.True(t, tt.After(bt))
			assert.True(t, at.After(tt))
			assert.True(t, ap.After(at))

			assert.Equal(t, ResultFailed, tr.Result)
			assert.Len(t, tr.Msgs, 1)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "all helpers succeed, test panics",
		beforePackage: succeeds(&bp),
		beforeTest:    succeeds(&bt),
		afterTest:     succeeds(&at),
		afterPackage:  succeeds(&ap),
		test:          panics(&tt),
		rootResult:    ResultFailed,
		validate: func(t *testing.T, tr TestResult) {
			assert.NotZero(t, bp)
			assert.True(t, bt.After(bp))
			assert.True(t, tt.After(bt))
			assert.True(t, at.After(tt))
			assert.True(t, ap.After(at))

			assert.Equal(t, ResultFailed, tr.Result)
			assert.Len(t, tr.Msgs, 1)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "only successful before package, test passes",
		beforePackage: succeeds(&bp),
		beforeTest:    nil,
		afterTest:     nil,
		afterPackage:  nil,
		test:          succeeds(&tt),
		rootResult:    ResultPassed,
		validate: func(t *testing.T, tr TestResult) {
			assert.NotZero(t, bp)
			assert.Zero(t, bt)
			assert.True(t, tt.After(bp))
			assert.Zero(t, at)
			assert.Zero(t, ap)

			assert.Equal(t, ResultPassed, tr.Result)
			assert.Len(t, tr.Msgs, 0)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "only successful before test, test passes",
		beforePackage: nil,
		beforeTest:    succeeds(&bt),
		afterTest:     nil,
		afterPackage:  nil,
		test:          succeeds(&tt),
		rootResult:    ResultPassed,
		validate: func(t *testing.T, tr TestResult) {
			assert.Zero(t, bp)
			assert.NotZero(t, bt)
			assert.True(t, tt.After(bt))
			assert.Zero(t, at)
			assert.Zero(t, ap)

			assert.Equal(t, ResultPassed, tr.Result)
			assert.Len(t, tr.Msgs, 0)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "only successful after test, test passes",
		beforePackage: nil,
		beforeTest:    nil,
		afterTest:     succeeds(&at),
		afterPackage:  nil,
		test:          succeeds(&tt),
		rootResult:    ResultPassed,
		validate: func(t *testing.T, tr TestResult) {
			assert.Zero(t, bp)
			assert.Zero(t, bt)
			assert.NotZero(t, tt)
			assert.True(t, at.After(tt))
			assert.Zero(t, ap)

			assert.Equal(t, ResultPassed, tr.Result)
			assert.Len(t, tr.Msgs, 0)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "only successful after package, test passes",
		beforePackage: nil,
		beforeTest:    nil,
		afterTest:     nil,
		afterPackage:  succeeds(&ap),
		test:          succeeds(&tt),
		rootResult:    ResultPassed,
		validate: func(t *testing.T, tr TestResult) {
			assert.Zero(t, bp)
			assert.Zero(t, bt)
			assert.NotZero(t, tt)
			assert.Zero(t, at)
			assert.True(t, ap.After(tt))

			assert.Equal(t, ResultPassed, tr.Result)
			assert.Len(t, tr.Msgs, 0)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "panic before package does not call before/after test or test but calls after package",
		beforePackage: panics(&bp),
		beforeTest:    succeeds(&bt),
		afterTest:     succeeds(&at),
		afterPackage:  succeeds(&ap),
		test:          succeeds(&tt),
		rootResult:    ResultFailed,
		validate: func(t *testing.T, tr TestResult) {
			assert.NotZero(t, bp)
			assert.Zero(t, bt)
			assert.Zero(t, tt)
			assert.Zero(t, at)
			assert.True(t, ap.After(bp))

			assert.Equal(t, ResultFailed, tr.Result)
			assert.Len(t, tr.Msgs, 1)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "panic before package does not call before/after test or test but calls after package which also panics",
		beforePackage: panics(&bp),
		beforeTest:    succeeds(&bt),
		afterTest:     succeeds(&at),
		afterPackage:  panics(&ap),
		test:          succeeds(&tt),
		rootResult:    ResultFailed,
		validate: func(t *testing.T, tr TestResult) {
			assert.NotZero(t, bp)
			assert.Zero(t, bt)
			assert.Zero(t, tt)
			assert.Zero(t, at)
			assert.True(t, ap.After(bp))

			assert.Equal(t, ResultFailed, tr.Result)
			assert.Len(t, tr.Msgs, 2)

			assert.Nil(t, subtestResult)
		},
	},
	{
		name:          "panic before test does not call test but calls after test",
		beforePackage: succeeds(&bp),
		beforeTest:    panics(&bt),
		afterTest:     succeeds(&at),
		afterPackage:  succeeds(&ap),
		test:          succeeds(&tt),
		rootResult:    ResultFailed,
		validate: func(t *testing.T, tr TestResult) {
			assert.NotZero(t, bp)
			assert.True(t, bt.After(bp))
			assert.Zero(t, tt)
			assert.True(t, at.After(bt))
			assert.True(t, ap.After(at))

			assert.Equal(t, ResultFailed, tr.Result)
			assert.Len(t, tr.Msgs, 1)

			assert.Nil(t, subtestResult)
		},
	},
}

func TestRun(t *testing.T) {
	for _, tc := range runTCs {
		t.Run(tc.name, func(t *testing.T) {
			// reset everything
			instance = testy{}
			bp = time.Time{}
			bt = time.Time{}
			at = time.Time{}
			ap = time.Time{}
			tt = time.Time{}
			subtestResult = nil

			// set up everything we need

			if tc.beforePackage != nil {
				BeforePackage(tc.beforePackage)
			}
			if tc.beforeTest != nil {
				BeforeTest(tc.beforeTest)
			}
			if tc.afterTest != nil {
				AfterTest(tc.afterTest)
			}
			if tc.afterPackage != nil {
				AfterPackage(tc.afterPackage)
			}

			Test(tc.name, tc.test)

			// We can't test RunAsTest since it needs a real testing.T, but if we use *our* testing.T, the "test
			// failures" test cases will cause the actual test to fail. There's probably some way to decouple this, but
			// I'm not able to think of it right now.

			res := Run()
			assert.Equal(t, tc.rootResult, res.Result)

			require.Len(t, res.Subtests, 1)
			require.Len(t, res.Subtests[0].Subtests, 1)
			// look into the test suite results and the package results
			tc.validate(t, res.Subtests[0].Subtests[0])
		})
	}
}

func TestRunWithOptionsRunsPackagesConcurrentlyAndPreservesOrder(t *testing.T) {
	instance = testy{
		tests: orderedmap.OrderedMap[string, *testPkg]{},
	}
	defer func() {
		instance = testy{}
	}()

	started := make(chan string, 3)
	release := make(chan struct{})
	var running int32
	var maxRunning int32

	for i := 0; i < 3; i++ {
		pkg := fmt.Sprintf("github.com/gametimesf/testy/concurrency/pkg%d", i)
		testName := fmt.Sprintf("test%d", i)
		instance.tests[pkg] = &testPkg{
			name:  pkg,
			tests: orderedmap.OrderedMap[string, testCase]{},
		}
		instance.tests[pkg].tests[testName] = testCase{
			Package: pkg,
			Name:    testName,
			tester: func(t TestingT) {
				nowRunning := atomic.AddInt32(&running, 1)
				for {
					max := atomic.LoadInt32(&maxRunning)
					if nowRunning <= max || atomic.CompareAndSwapInt32(&maxRunning, max, nowRunning) {
						break
					}
				}
				started <- pkg
				<-release
				atomic.AddInt32(&running, -1)
			},
		}
	}

	done := make(chan TestResult, 1)
	go func() {
		done <- RunWithOptions(RunOptions{PackageConcurrency: 2})
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for package %d to start", i+1)
		}
	}

	select {
	case pkg := <-started:
		t.Fatalf("package %s started before a concurrency slot was released", pkg)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	var res TestResult
	select {
	case res = <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for concurrent run to finish")
	}

	assert.LessOrEqual(t, atomic.LoadInt32(&maxRunning), int32(2))
	assert.Equal(t, ResultPassed, res.Result)
	require.Len(t, res.Subtests, 3)
	for i, pkgResult := range res.Subtests {
		assert.Equal(t, fmt.Sprintf("github.com/gametimesf/testy/concurrency/pkg%d", i), pkgResult.Package)
		require.Len(t, pkgResult.Subtests, 1)
		assert.Equal(t, fmt.Sprintf("test%d", i), pkgResult.Subtests[0].Name)
	}
}

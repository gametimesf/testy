package testy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// know that before/after package/test and the test itself have run and when they were run
var bp, bt, at, ap, tt time.Time

func succeeds(ts *time.Time) Tester {
	return func(TestingT) {
		*ts = time.Now()
	}
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
		t.Run("subtest", tester)
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

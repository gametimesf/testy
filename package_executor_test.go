package testy

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gametimesf/testy/internal/orderedmap"
)

// Both shapes must overlap: ordinary cases in different packages at the serial
// default, and audited cases inside one package when explicitly enabled.
func TestPackageExecutorOverlap(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		limit, packages, cases int
		concurrent             bool
	}{
		{"ordinary packages", 1, 2, 1, false},
		{"audited cases", 2, 1, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := NewPackageCaseExecutor(2, tc.limit)
			entered := make(chan struct{}, 2)
			release := make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			results := make(chan TestResult, tc.packages)
			for i := 0; i < tc.packages; i++ {
				pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
				for j := 0; j < tc.cases; j++ {
					name := string(rune('a' + j))
					pkg.tests[name] = testCase{Name: name, Concurrent: tc.concurrent, tester: func(tt TestingT) {
						Cleanup(tt, func() { entered <- struct{}{}; <-release })
					}}
				}
				go func(name string, pkg *testPkg) {
					results <- runPackageWithOptions(context.Background(), name, pkg, RunOptions{CaseExecutor: executor})
				}(string(rune('a'+i)), pkg)
			}
			for i := 0; i < 2; i++ {
				select {
				case <-entered:
				case <-time.After(2 * time.Second):
					t.Fatal("expected lifecycles did not overlap")
				}
			}
			once.Do(func() { close(release) })
			for i := 0; i < tc.packages; i++ {
				select {
				case r := <-results:
					if r.Result != ResultPassed {
						t.Fatalf("result=%+v", r)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("lifecycle did not drain")
				}
			}
		})
	}
}

func TestPackageExecutorWorkerBudgetIncludesHooksAndCleanup(t *testing.T) {
	executor := NewPackageCaseExecutor(2, 2)
	var active, maximum atomic.Int32
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	begin := func() {
		n := active.Add(1)
		for old := maximum.Load(); n > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, n) {
				break
			}
		}
	}
	end := func() { active.Add(-1) }
	results := make(chan TestResult, 4)
	for i := 0; i < 4; i++ {
		pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
		pkg.BeforePackage = func(TestingT) { begin(); end() }
		pkg.AfterPackage = func(TestingT) { begin(); end() }
		pkg.BeforeTest = func(TestingT) { begin() }
		pkg.AfterTest = func(TestingT) { end() }
		for _, name := range []string{"a", "b"} {
			pkg.tests[name] = testCase{Name: name, Concurrent: true, tester: func(tt TestingT) {
				Cleanup(tt, func() {
					select {
					case entered <- struct{}{}:
					default:
					}
					<-release
				})
			}}
		}
		go func(name string) {
			results <- runPackageWithOptions(context.Background(), name, pkg, RunOptions{CaseExecutor: executor})
		}(string(rune('a' + i)))
	}
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("budget was not utilized")
		}
	}
	once.Do(func() { close(release) })
	for i := 0; i < 4; i++ {
		select {
		case r := <-results:
			if r.Result != ResultPassed {
				t.Fatalf("result=%+v", r)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("work did not drain")
		}
	}
	if active.Load() != 0 || maximum.Load() != 2 {
		t.Fatalf("active=%d maximum=%d", active.Load(), maximum.Load())
	}
}

func TestPackageExecutorSamePackageWaitCancelsWithoutRunningHooks(t *testing.T) {
	executor := NewPackageCaseExecutor(4, 2)
	_, release, err := executor.acquirePackage(context.Background(), "shared", false)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	var invoked atomic.Int32
	pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
	pkg.BeforePackage = func(TestingT) { invoked.Add(1) }
	pkg.AfterPackage = pkg.BeforePackage
	pkg.tests["ordinary"] = testCase{Name: "ordinary", tester: pkg.BeforePackage}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result := runPackageWithOptions(ctx, "shared", pkg, RunOptions{CaseExecutor: executor})
	if result.Result != ResultFailed || len(result.Subtests) != 1 || invoked.Load() != 0 {
		t.Fatalf("result=%+v invoked=%d", result, invoked.Load())
	}
	// A different package is still independently admitted.
	_, otherRelease, err := executor.acquirePackage(context.Background(), "other", false)
	if err != nil {
		t.Fatal(err)
	}
	otherRelease()
}

func TestPackageExecutorBudgetCancellationRollsBackPackageSlots(t *testing.T) {
	executor := NewPackageCaseExecutor(1, 2)
	first, releaseFirst, err := executor.acquirePackage(context.Background(), "first", false)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFirst()
	second, releaseSecond, err := executor.acquirePackage(context.Background(), "second", false)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseSecond()
	hold, err := first.acquire(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = second.acquire(ctx, false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected budget wait cancellation, got %v", err)
	}
	if len(second.slots) != 0 || len(second.gate) != 0 {
		t.Fatal("cancellation leaked package permits")
	}
	hold()
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	again, err := second.acquire(ctx2, false)
	if err != nil {
		t.Fatal(err)
	}
	again()
	if len(executor.budget) != 0 {
		t.Fatal("worker budget leaked")
	}
}

func TestPackageExecutorInvocationOwnsAfterPackage(t *testing.T) {
	executor := NewPackageCaseExecutor(4, 2)
	afterStarted, releaseAfter := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(releaseAfter) })
	var setups atomic.Int32
	pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{"case": {Name: "case", Concurrent: true, tester: func(TestingT) {}}}}
	pkg.BeforePackage = func(TestingT) { setups.Add(1) }
	pkg.AfterPackage = func(TestingT) {
		if setups.Load() == 1 {
			close(afterStarted)
			<-releaseAfter
		}
	}
	first := make(chan TestResult, 1)
	go func() {
		first <- runPackageWithOptions(context.Background(), "shared", pkg, RunOptions{CaseExecutor: executor})
	}()
	select {
	case <-afterStarted:
	case <-time.After(time.Second):
		t.Fatal("first invocation never reached teardown")
	}
	// A deadline gives the second invocation a chance to contend while teardown
	// owns the gate; neither of its hooks may run.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	canceled := runPackageWithOptions(ctx, "shared", pkg, RunOptions{CaseExecutor: executor})
	if canceled.Result != ResultFailed || setups.Load() != 1 {
		t.Fatalf("teardown gate violated: %+v setups=%d", canceled, setups.Load())
	}
	second := make(chan TestResult, 1)
	go func() {
		second <- runPackageWithOptions(context.Background(), "shared", pkg, RunOptions{CaseExecutor: executor})
	}()
	once.Do(func() { close(releaseAfter) })
	for _, done := range []chan TestResult{first, second} {
		select {
		case r := <-done:
			if r.Result != ResultPassed {
				t.Fatalf("result=%+v", r)
			}
		case <-time.After(time.Second):
			t.Fatal("invocation did not drain")
		}
	}
	if setups.Load() != 2 {
		t.Fatalf("setups=%d", setups.Load())
	}
}

func TestPackageExecutorOrdinaryCaseExcludesConcurrentSiblings(t *testing.T) {
	executor := NewPackageCaseExecutor(4, 2)
	var active atomic.Int32
	pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
	pkg.BeforeTest = func(TestingT) { active.Add(1) }
	pkg.AfterTest = func(TestingT) { active.Add(-1) }
	for _, name := range []string{"a", "c"} {
		pkg.tests[name] = testCase{Name: name, Concurrent: true, tester: func(tt TestingT) { Cleanup(tt, func() { time.Sleep(time.Millisecond) }) }}
	}
	pkg.tests["b"] = testCase{Name: "b", tester: func(tt TestingT) {
		if active.Load() != 1 {
			tt.Errorf("ordinary case overlapped: active=%d", active.Load())
		}
		Cleanup(tt, func() {
			if active.Load() != 1 {
				tt.Errorf("ordinary cleanup overlapped: active=%d", active.Load())
			}
		})
	}}
	r := runPackageWithOptions(context.Background(), "mixed", pkg, RunOptions{CaseExecutor: executor})
	if r.Result != ResultPassed || active.Load() != 0 {
		t.Fatalf("result=%+v active=%d", r, active.Load())
	}
}

// Same-package invocations overlap only when the package is declared, hookless
// and all-ConcurrentTest. Each overlapping invocation keeps its own package limit
// and every invocation shares the worker budget.
func TestPackageExecutorSamePackageInvocations(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		total, limit, invocations      int
		declared, before, after, plain bool
		wantActive                     int32
		wantMsg                        string
	}{
		{name: "declared package overlaps within its package limit", total: 4, limit: 1, invocations: 2, declared: true, wantActive: 2},
		{name: "package limit applies per invocation", total: 8, limit: 2, invocations: 2, declared: true, wantActive: 4},
		{name: "shared budget bounds overlapping invocations", total: 2, limit: 2, invocations: 3, declared: true, wantActive: 2},
		{name: "undeclared package serializes", total: 4, limit: 1, invocations: 2, wantActive: 1},
		{name: "BeforePackage serializes", total: 4, limit: 1, invocations: 2, declared: true, before: true, wantActive: 1, wantMsg: "BeforePackage or AfterPackage"},
		{name: "AfterPackage serializes", total: 4, limit: 1, invocations: 2, declared: true, after: true, wantActive: 1, wantMsg: "BeforePackage or AfterPackage"},
		{name: "ordinary case serializes", total: 4, limit: 1, invocations: 2, declared: true, plain: true, wantActive: 1, wantMsg: "case b is not a ConcurrentTest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := NewPackageCaseExecutor(tc.total, tc.limit)
			var active, maximum atomic.Int32
			entered := make(chan struct{}, 16)
			release := make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			tester := func(TestingT) {
				n := active.Add(1)
				defer active.Add(-1)
				for old := maximum.Load(); n > old; old = maximum.Load() {
					if maximum.CompareAndSwap(old, n) {
						break
					}
				}
				entered <- struct{}{}
				<-release
			}
			pkg := &testPkg{ConcurrentInvocations: tc.declared, tests: orderedmap.OrderedMap[string, testCase]{
				"a": {Name: "a", Concurrent: true, tester: tester},
				"b": {Name: "b", Concurrent: !tc.plain, tester: tester},
			}}
			if tc.before {
				pkg.BeforePackage = func(TestingT) {}
			}
			if tc.after {
				pkg.AfterPackage = func(TestingT) {}
			}
			results := make(chan TestResult, tc.invocations)
			for i := 0; i < tc.invocations; i++ {
				go func() {
					results <- runPackageWithOptions(context.Background(), "shared", pkg, RunOptions{CaseExecutor: executor})
				}()
			}
			for i := int32(0); i < tc.wantActive; i++ {
				select {
				case <-entered:
				case <-time.After(2 * time.Second):
					t.Fatalf("only %d of %d expected lifecycles were admitted", i, tc.wantActive)
				}
			}
			select {
			case <-entered:
				t.Fatalf("admitted more than %d lifecycles", tc.wantActive)
			case <-time.After(50 * time.Millisecond):
			}
			once.Do(func() { close(release) })
			for i := 0; i < tc.invocations; i++ {
				select {
				case r := <-results:
					if r.Result != ResultPassed || len(r.Subtests) != 2 {
						t.Fatalf("result=%+v", r)
					}
					probe, want := tc.wantMsg, tc.wantMsg != ""
					if !want {
						probe = "declaration ignored"
					}
					if hasMsg(r.Msgs, probe) != want {
						t.Fatalf("diagnostic %q want=%v msgs=%+v", probe, want, r.Msgs)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("invocations did not drain")
				}
			}
			if maximum.Load() != tc.wantActive || len(executor.budget) != 0 {
				t.Fatalf("maximum=%d budget=%d", maximum.Load(), len(executor.budget))
			}
		})
	}
}

// A canceled waiter runs no hooks or cases and returns every held permit,
// whether it waited for the package gate or for the shared worker budget.
func TestPackageExecutorSamePackageWaitCancellationRollsBack(t *testing.T) {
	for _, tc := range []struct {
		name   string
		before bool
		hold   func(*testing.T, *CaseExecutor) func()
	}{
		{name: "overlapping invocation waiting on worker budget", hold: func(t *testing.T, executor *CaseExecutor) func() {
			scope, releaseScope, err := executor.acquirePackage(context.Background(), "other", false)
			if err != nil {
				t.Fatal(err)
			}
			releaseCase, err := scope.acquire(context.Background(), false)
			if err != nil {
				t.Fatal(err)
			}
			return func() { releaseCase(); releaseScope() }
		}},
		{name: "ineligible declaration waiting on package gate", before: true, hold: func(t *testing.T, executor *CaseExecutor) func() {
			_, releaseScope, err := executor.acquirePackage(context.Background(), "shared", false)
			if err != nil {
				t.Fatal(err)
			}
			return releaseScope
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := NewPackageCaseExecutor(1, 1)
			var invoked atomic.Int32
			invoke := func(TestingT) { invoked.Add(1) }
			pkg := &testPkg{ConcurrentInvocations: true, tests: orderedmap.OrderedMap[string, testCase]{
				"a": {Name: "a", Concurrent: true, tester: invoke},
				"b": {Name: "b", Concurrent: true, tester: invoke},
			}}
			if tc.before {
				pkg.BeforePackage = invoke
			}
			releaseHold := tc.hold(t, executor)
			held := len(executor.budget)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			r := runPackageWithOptions(ctx, "shared", pkg, RunOptions{CaseExecutor: executor})
			if r.Result != ResultFailed || len(r.Subtests) != 2 || invoked.Load() != 0 {
				t.Fatalf("result=%+v invoked=%d", r, invoked.Load())
			}
			for _, child := range r.Subtests {
				if child.Result != ResultFailed {
					t.Fatalf("canceled case reported %+v", child)
				}
			}
			if len(executor.budget) != held {
				t.Fatalf("budget=%d want %d", len(executor.budget), held)
			}
			releaseHold()
			again := runPackageWithOptions(context.Background(), "shared", pkg, RunOptions{CaseExecutor: executor})
			if again.Result != ResultPassed || len(executor.budget) != 0 {
				t.Fatalf("result=%+v budget=%d", again, len(executor.budget))
			}
		})
	}
}

func hasMsg(msgs []Msg, substr string) bool {
	for _, m := range msgs {
		if strings.Contains(m.Msg, substr) {
			return true
		}
	}
	return false
}

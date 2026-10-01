package testy

import (
	"context"
	"errors"
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
	_, release, err := executor.acquirePackage(context.Background(), "shared")
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
	_, otherRelease, err := executor.acquirePackage(context.Background(), "other")
	if err != nil {
		t.Fatal(err)
	}
	otherRelease()
}

func TestPackageExecutorBudgetCancellationRollsBackPackageSlots(t *testing.T) {
	executor := NewPackageCaseExecutor(1, 2)
	first, releaseFirst, err := executor.acquirePackage(context.Background(), "first")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFirst()
	second, releaseSecond, err := executor.acquirePackage(context.Background(), "second")
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

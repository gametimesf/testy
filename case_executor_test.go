package testy

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gametimesf/testy/internal/orderedmap"
)

func TestCaseExecutorBoundsAndCancellation(t *testing.T) {
	executor := NewCaseExecutor(2)
	release, err := executor.acquire(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	release2, err := executor.acquire(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := executor.acquire(ctx, true); err != context.Canceled {
		t.Fatalf("acquire = %v", err)
	}
	release()
	release2()
	exclusive, err := executor.acquire(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if _, err := executor.acquire(ctx, true); err != context.Canceled {
		t.Fatalf("exclusive did not block: %v", err)
	}
	exclusive()
	again, err := executor.acquire(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	again()
}

func TestConcurrentCasesHoldSharedPermitThroughCleanupAndHooks(t *testing.T) {
	executor := NewCaseExecutor(2)
	var active, maximum atomic.Int32
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	after := make(chan struct{}, 4)
	makePackage := func() *testPkg {
		pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
		pkg.BeforeTest = func(TestingT) {
			n := active.Add(1)
			for old := maximum.Load(); n > old; old = maximum.Load() {
				if maximum.CompareAndSwap(old, n) {
					break
				}
			}
		}
		pkg.AfterTest = func(TestingT) { active.Add(-1); after <- struct{}{} }
		for _, name := range []string{"a", "b"} {
			pkg.tests[name] = testCase{Name: name, Concurrent: true, tester: func(tt TestingT) {
				Cleanup(tt, func() { entered <- struct{}{}; <-release })
				tt.Run("step one", func(TestingT) {})
				tt.Run("step two", func(TestingT) {})
			}}
		}
		return pkg
	}
	results := make(chan TestResult, 2)
	for _, name := range []string{"pkg-a", "pkg-b"} {
		go func(name string) {
			results <- runPackageWithOptions(context.Background(), name, makePackage(), RunOptions{CaseExecutor: executor})
		}(name)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("independent cases did not overlap")
		}
	}
	select {
	case <-after:
		t.Fatal("AfterTest before cleanup")
	default:
	}
	if active.Load() != 2 {
		t.Fatalf("active=%d", active.Load())
	}
	close(release)
	for i := 0; i < 2; i++ {
		r := <-results
		if r.Result != ResultPassed || len(r.Subtests) != 2 || r.Subtests[0].Name != "a" {
			t.Fatalf("result=%+v", r)
		}
	}
	if active.Load() != 0 || maximum.Load() != 2 {
		t.Fatalf("active=%d max=%d", active.Load(), maximum.Load())
	}
}

func TestSerialCaseExcludesOtherPackages(t *testing.T) {
	executor := NewCaseExecutor(2)
	release, err := executor.acquire(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{}, 1)
	pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{"independent": {Name: "independent", Concurrent: true, tester: func(TestingT) { entered <- struct{}{} }}}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan TestResult, 1)
	go func() { done <- runPackageWithOptions(ctx, "p", pkg, RunOptions{CaseExecutor: executor}) }()
	cancel()
	result := <-done
	release()
	select {
	case <-entered:
		t.Fatal("canceled queued case executed")
	default:
	}
	if result.Result != ResultFailed || len(result.Subtests) != 1 {
		t.Fatalf("canceled work must not pass %+v", result)
	}
}

func TestCaseCancellationDrainsCleanupBeforeReturn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	cleanupStarted := make(chan struct{})
	releaseCleanup := make(chan struct{})
	pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{"a": {Name: "a", Concurrent: true, tester: func(tt TestingT) {
		Cleanup(tt, func() { close(cleanupStarted); <-releaseCleanup })
		close(started)
		<-Context(tt).Done()
	}}}}
	done := make(chan TestResult, 1)
	go func() { done <- runPackageWithOptions(ctx, "p", pkg, RunOptions{CaseExecutor: NewCaseExecutor(1)}) }()
	<-started
	cancel()
	<-cleanupStarted
	select {
	case <-done:
		t.Fatal("runner returned before cleanup drained")
	default:
	}
	close(releaseCleanup)
	if result := <-done; result.Result != ResultFailed {
		t.Fatalf("canceled case passed %+v", result)
	}
}

func TestConcurrentCaseFailureIsolationAndOrderedChildren(t *testing.T) {
	var mu sync.Mutex
	var order []string
	pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{
		"a panic": {Name: "a panic", Concurrent: true, tester: func(TestingT) { panic("isolated") }},
		"b fatal": {Name: "b fatal", Concurrent: true, tester: func(tt TestingT) { tt.Fatal("isolated") }},
		"c skip":  {Name: "c skip", Concurrent: true, tester: func(tt TestingT) { Skipf(tt, "prerequisite") }},
		"d steps": {Name: "d steps", Concurrent: true, tester: func(tt TestingT) {
			for _, step := range []string{"one", "two"} {
				step := step
				tt.Run(step, func(TestingT) { mu.Lock(); order = append(order, step); mu.Unlock() })
			}
		}},
	}}
	r := runPackageWithOptions(context.Background(), "p", pkg, RunOptions{CaseExecutor: NewCaseExecutor(2)})
	if r.Result != ResultFailed || len(r.Subtests) != 4 || r.Subtests[2].Result != ResultSkipped || r.Subtests[3].Result != ResultPassed || !reflect.DeepEqual(order, []string{"one", "two"}) {
		t.Fatalf("result=%+v order=%v", r, order)
	}
}

func TestNativeAfterPackageWaitsForParallelChildren(t *testing.T) {
	prev := instance
	defer func() { instance = prev }()
	alive := true
	pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
	pkg.AfterPackage = func(TestingT) { alive = false }
	pkg.tests["case"] = testCase{Name: "case", tester: func(tt TestingT) {
		tt.Parallel()
		if !alive {
			tt.Fatal("AfterPackage ran before parallel case resumed")
		}
	}}
	instance = testy{tests: orderedmap.OrderedMap[string, *testPkg]{"probe": pkg}}
	t.Run("suite", RunAsTest)
	if alive {
		t.Fatal("AfterPackage did not run")
	}
}

func TestConcurrentRegistrationAndSnapshot(t *testing.T) {
	previous := instance
	defer func() { instance = previous }()
	instance = testy{}
	ConcurrentTest("z independent", func(TestingT) {})
	Test("a serial", func(TestingT) {})
	specs, err := ListCases("github.com/gametimesf/testy")
	if err != nil || !reflect.DeepEqual(specs, []CaseSpec{{Name: "a_serial", Index: 0}, {Name: "z_independent", Index: 1, Concurrent: true}}) {
		t.Fatalf("specs=%+v err=%v", specs, err)
	}
	specs[0].Name = "mutated"
	again, err := ListCases("github.com/gametimesf/testy")
	if err != nil || again[0].Name != "a_serial" {
		t.Fatalf("snapshot aliased registry: %+v %v", again, err)
	}
	if _, err := ListCases("missing"); err == nil {
		t.Fatal("missing package accepted")
	}
}

func TestNativeConcurrentRegistrationWaitsForCleanup(t *testing.T) {
	previous := instance
	defer func() { instance = previous }()
	var active atomic.Int32
	var completed atomic.Int32
	pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
	pkg.BeforeTest = func(TestingT) { active.Add(1) }
	pkg.AfterTest = func(tt TestingT) {
		if completed.Load() == 0 {
			tt.Fatal("AfterTest preceded cleanup")
		}
		active.Add(-1)
	}
	pkg.AfterPackage = func(tt TestingT) {
		if active.Load() != 0 || completed.Load() != 2 {
			tt.Fatal("AfterPackage preceded case drain")
		}
	}
	for _, name := range []string{"a", "b"} {
		pkg.tests[name] = testCase{Name: name, Concurrent: true, tester: func(tt TestingT) { Cleanup(tt, func() { completed.Add(1) }) }}
	}
	instance = testy{tests: orderedmap.OrderedMap[string, *testPkg]{"probe": pkg}}
	t.Run("suite", RunAsTest)
}

func TestCaseExecutorSerialFallbackAndCanonicalAdmission(t *testing.T) {
	for _, executor := range []*CaseExecutor{nil, NewCaseExecutor(1)} {
		var order []string
		pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
		for _, name := range []string{"c", "b", "a"} {
			name := name
			pkg.tests[name] = testCase{Name: name, Concurrent: true, tester: func(tt TestingT) {
				order = append(order, name)
				Cleanup(tt, func() { order = append(order, name+" cleanup") })
			}}
		}
		result := runPackageWithOptions(context.Background(), "p", pkg, RunOptions{CaseExecutor: executor})
		if result.Result != ResultPassed || !reflect.DeepEqual(order, []string{"a", "a cleanup", "b", "b cleanup", "c", "c cleanup"}) {
			t.Fatalf("order=%v result=%+v", order, result)
		}
		for _, child := range result.Subtests {
			if child.QueueDur < 0 || child.Started.IsZero() {
				t.Fatalf("missing timing %+v", child)
			}
		}
	}
}

func TestPackageHooksWaitForSharedExecutor(tt *testing.T) {
	executor := NewCaseExecutor(2)
	release, err := executor.acquire(context.Background(), true)
	if err != nil {
		tt.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err = runPackageHook(ctx, executor, "before package", func(TestingT) { called = true }, &t{})
	release()
	if err != context.Canceled || called {
		tt.Fatalf("canceled hook ran=%v err=%v", called, err)
	}
}

func TestPackageHookErrorDiagnosticsAreRetained(t *testing.T) {
	for _, phase := range []string{"before", "after"} {
		t.Run(phase, func(t *testing.T) {
			pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{"a": {Name: "a", Concurrent: true, tester: func(TestingT) {}}}}
			hook := func(tt TestingT) { tt.Errorf("diagnostic-marker-%s", phase) }
			if phase == "before" {
				pkg.BeforePackage = hook
			} else {
				pkg.AfterPackage = hook
			}
			result := runPackageWithOptions(context.Background(), "p", pkg, RunOptions{CaseExecutor: NewCaseExecutor(2)})
			if result.Result != ResultFailed || len(result.Msgs) != 1 || !strings.Contains(result.Msgs[0].Msg, "diagnostic-marker-"+phase) {
				t.Fatalf("package hook failure lost diagnostic %+v", result)
			}
		})
	}
}

func TestLiveExclusiveCaseCannotOverlapIndependentCases(t *testing.T) {
	executor := NewCaseExecutor(2)
	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	thirdStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	defer func() {
		select {
		case <-releaseFirst:
		default:
			close(releaseFirst)
		}
		select {
		case <-releaseSecond:
		default:
			close(releaseSecond)
		}
	}()
	pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{
		"a": {Name: "a", Concurrent: true, tester: func(tt TestingT) { close(firstStarted); Cleanup(tt, func() { <-releaseFirst }) }},
		"b": {Name: "b", tester: func(tt TestingT) { close(secondStarted); Cleanup(tt, func() { <-releaseSecond }) }},
		"c": {Name: "c", Concurrent: true, tester: func(TestingT) { close(thirdStarted) }},
	}}
	done := make(chan TestResult, 1)
	go func() {
		done <- runPackageWithOptions(context.Background(), "p", pkg, RunOptions{CaseExecutor: executor})
	}()
	<-firstStarted
	select {
	case <-secondStarted:
		t.Fatal("exclusive case overlapped first cleanup")
	case <-time.After(30 * time.Millisecond):
	}
	close(releaseFirst)
	<-secondStarted
	select {
	case <-thirdStarted:
		t.Fatal("independent case overlapped exclusive cleanup")
	case <-time.After(30 * time.Millisecond):
	}
	close(releaseSecond)
	if result := <-done; result.Result != ResultPassed {
		t.Fatalf("result %+v", result)
	}
}

func TestCancellationReleasesPartialExclusiveAdmission(t *testing.T) {
	executor := NewCaseExecutor(2)
	held, err := executor.acquire(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waiting := make(chan struct{})
	exclusiveDone := make(chan error, 1)
	go func() {
		close(waiting)
		release, err := executor.acquire(ctx, false)
		if err == nil {
			release()
		}
		exclusiveDone <- err
	}()
	<-waiting
	// The exclusive acquisition cannot complete while the first shared owner lives.
	select {
	case err := <-exclusiveDone:
		t.Fatalf("exclusive admitted early: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	cancel()
	if err := <-exclusiveDone; err != context.Canceled {
		t.Fatalf("canceled acquisition: %v", err)
	}
	// Still holding the first permit: a new shared owner must get the rolled-back
	// remaining permit, not block behind a canceled partial exclusive acquisition.
	nextCtx, nextCancel := context.WithTimeout(context.Background(), time.Second)
	defer nextCancel()
	release, err := executor.acquire(nextCtx, true)
	if err != nil {
		t.Fatalf("partial admission leaked: %v", err)
	}
	release()
}

package testy

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gametimesf/testy/internal/orderedmap"
)

func lifecycleExercise(t TestingT, mode string, record func(string)) {
	ctx := Context(t)
	Cleanup(t, func() { record("first") })
	Cleanup(t, func() {
		if ctx.Err() != context.Canceled {
			t.Errorf("context was not canceled before cleanup")
		}
		record("last")
		switch mode {
		case "cleanup-fatal":
			t.Fatal("cleanup failure")
		case "cleanup-panic":
			panic("cleanup failure")
		case "nested-cleanup":
			Cleanup(t, func() { record("nested") })
		}
	})
	t.Run("child", func(child TestingT) {
		childCtx := Context(child)
		if childCtx == ctx {
			child.Fatal("child must have its own context")
		}
		Cleanup(child, func() { record("child") })
	})
	if ctx.Err() != nil {
		t.Errorf("child cleanup canceled parent")
	}
	switch mode {
	case "fatal":
		t.Fatal("body failure")
	case "panic":
		panic("body failure")
	}
}

func TestHostedLifecycle(t *testing.T) {
	for _, mode := range []string{"normal", "fatal", "panic", "cleanup-fatal", "cleanup-panic", "nested-cleanup"} {
		t.Run(mode, func(t *testing.T) {
			var got []string
			result := runTest("lifecycle", mode, func(tt TestingT) {
				lifecycleExercise(tt, mode, func(s string) { got = append(got, s) })
			})
			want := []string{"child", "last", "first"}
			if mode == "nested-cleanup" {
				want = []string{"child", "last", "nested", "first"}
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("cleanup order: got %v want %v", got, want)
			}
			failed := mode != "normal" && mode != "nested-cleanup"
			if (result.Result == ResultFailed) != failed {
				t.Fatalf("unexpected result %s", result.Result)
			}
		})
	}
}

// A subprocess exercises native testing.T fatal/panic paths without making the
// parent suite fail. The same behavior is checked against the hosted runner above.
func TestNativeLifecycle(t *testing.T) {
	if mode := os.Getenv("TESTY_LIFECYCLE_MODE"); mode != "" {
		lifecycleExercise(newTWrapper(t, context.Background()), mode, func(s string) { t.Log("cleanup-marker:" + s) })
		return
	}
	for _, mode := range []string{"normal", "fatal", "panic", "cleanup-fatal", "cleanup-panic", "nested-cleanup"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestNativeLifecycle$", "-test.v")
			cmd.Env = append(os.Environ(), "TESTY_LIFECYCLE_MODE="+mode)
			output, err := cmd.CombinedOutput()
			failed := mode != "normal" && mode != "nested-cleanup"
			if (err != nil) != failed {
				t.Fatalf("exit error=%v: %s", err, output)
			}
			markers := []string{"child", "last", "first"}
			if mode == "nested-cleanup" {
				markers = []string{"child", "last", "nested", "first"}
			}
			text := string(output)
			for _, marker := range markers {
				i := strings.Index(text, "cleanup-marker:"+marker)
				if i < 0 {
					t.Fatalf("missing or out-of-order %s: %s", marker, output)
				}
				text = text[i+len("cleanup-marker:"+marker):]
			}
			if strings.Contains(string(output), "context was not canceled") || strings.Contains(string(output), "child cleanup canceled") {
				t.Fatalf("context lifecycle: %s", output)
			}
		})
	}
}

// A subprocess receives a real SIGINT while a case runs: the case's context is
// canceled, its cleanup runs, the case that had not started is skipped, and the
// run still fails.
func TestNativeInterruptRunsCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs SIGINT")
	}
	if os.Getenv("TESTY_INTERRUPT_HELPER") != "" {
		pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
		pkg.tests["a-running"] = testCase{Name: "a-running", tester: func(tt TestingT) {
			Cleanup(tt, func() { tt.Log("cleanup-marker:running") })
			interruptSelf(tt)
			select {
			case <-Context(tt).Done():
				tt.Log("context-marker:canceled")
			case <-time.After(10 * time.Second):
				tt.Fatal("context not canceled by interrupt")
			}
		}}
		pkg.tests["b-later"] = testCase{Name: "b-later", tester: func(tt TestingT) {
			tt.Log("ran-marker:later")
		}}
		laterPkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
		laterPkg.BeforePackage = func(TestingT) { t.Log("hook-marker:later") }
		laterPkg.tests["c-later-package"] = testCase{Name: "c-later-package", tester: func(TestingT) {}}
		instance = testy{tests: orderedmap.OrderedMap[string, *testPkg]{"a-interrupt": pkg, "b-later": laterPkg}}
		RunAsTest(t)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestNativeInterruptRunsCleanup$", "-test.v")
	cmd.Env = append(os.Environ(), "TESTY_INTERRUPT_HELPER=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("interrupted run passed: %s", output)
	}
	text := string(output)
	for _, marker := range []string{"context-marker:canceled", "cleanup-marker:running", "--- SKIP: TestNativeInterruptRunsCleanup/b-later"} {
		if !strings.Contains(text, marker) {
			t.Fatalf("missing %q: %s", marker, output)
		}
	}
	for _, marker := range []string{"ran-marker:later", "hook-marker:later"} {
		if strings.Contains(text, marker) {
			t.Fatalf("%q after interrupt: %s", marker, output)
		}
	}
}

// After RunAsTest returns, testy must stop handling signals, so a later SIGINT
// takes the default action instead of canceling a finished run's context.
func TestNativeInterruptHandlerEndsWithRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs SIGINT")
	}
	if os.Getenv("TESTY_INTERRUPT_AFTER_RUN") != "" {
		pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
		pkg.tests["done"] = testCase{Name: "done", tester: func(TestingT) {}}
		instance = testy{tests: orderedmap.OrderedMap[string, *testPkg]{"done": pkg}}
		t.Run("suite", RunAsTest)
		interruptSelf(t)
		time.Sleep(2 * time.Second)
		t.Log("survived-marker")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestNativeInterruptHandlerEndsWithRun$", "-test.v")
	cmd.Env = append(os.Environ(), "TESTY_INTERRUPT_AFTER_RUN=1")
	output, err := cmd.CombinedOutput()
	if err == nil || strings.Contains(string(output), "survived-marker") {
		t.Fatalf("SIGINT after the run did not stop the process: %v: %s", err, output)
	}
}

func interruptSelf(t interface {
	Helper()
	Fatal(args ...any)
}) {
	t.Helper()
	self, err := os.FindProcess(os.Getpid())
	if err == nil {
		err = self.Signal(os.Interrupt)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestNativeParallelChildKeepsParentFixture(t *testing.T) {
	parent := newTWrapper(t, context.Background())
	alive := true
	Cleanup(parent, func() { alive = false })
	parent.Run("parallel", func(child TestingT) {
		child.Parallel()
		if !alive {
			child.Fatal("parent fixture cleaned before parallel child")
		}
	})
}

func TestLifecycleHooksRejected(t *testing.T) {
	for _, tt := range []TestingT{&tHelper{}, tWrapper{t: t}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("unsupported cleanup must panic")
				}
			}()
			Cleanup(tt, func() {})
		}()
	}
}

// Embedding preserves source compatibility with existing custom TestingT types.
type tHelper struct{ TestingT }

func (*tHelper) Helper() {}

func TestLifecycleAfterTestParity(t *testing.T) {
	for _, runner := range []string{"native", "hosted"} {
		t.Run(runner, func(t *testing.T) {
			var order []string
			pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
			pkg.BeforeTest = func(TestingT) { order = append(order, "before") }
			pkg.AfterTest = func(TestingT) { order = append(order, "after") }
			pkg.tests["case"] = testCase{Name: "case", tester: func(tt TestingT) {
				order = append(order, "body")
				Cleanup(tt, func() { order = append(order, "cleanup") })
			}}
			if runner == "hosted" {
				runPackage("lifecycle", pkg)
			} else {
				previous := instance
				instance = testy{tests: orderedmap.OrderedMap[string, *testPkg]{"lifecycle": pkg}}
				defer func() { instance = previous }()
				t.Run("suite", RunAsTest)
			}
			if !reflect.DeepEqual(order, []string{"before", "body", "cleanup", "after"}) {
				t.Fatalf("order %v", order)
			}
		})
	}
}

func TestLifecycleContextInheritance(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	wrapper := newTWrapper(t, parent)
	cancel()
	if Context(wrapper).Err() != context.Canceled {
		t.Fatal("wrapper did not inherit parent cancellation")
	}
	result := runTestContext(parent, "lifecycle", "canceled", func(tt TestingT) {
		if Context(tt).Err() != context.Canceled {
			tt.Fatal("hosted test did not inherit cancellation")
		}
		tt.Run("child", func(child TestingT) {
			if Context(child).Err() != context.Canceled {
				child.Fatal("child did not inherit cancellation")
			}
		})
	})
	if result.Result != ResultPassed {
		t.Fatalf("result %+v", result)
	}
	deadlineParent, stop := context.WithTimeout(context.Background(), time.Minute)
	defer stop()
	child := newTWrapper(t, deadlineParent)
	if _, ok := Context(child).Deadline(); !ok {
		t.Fatal("deadline was dropped")
	}
}

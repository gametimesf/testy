package testy

import (
	"context"
	"os"
	"os/exec"
	"reflect"
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

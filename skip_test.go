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

func exerciseSkip(tt TestingT, mode string, record func(string)) {
	ctx := Context(tt)
	Cleanup(tt, func() { record("first") })
	Cleanup(tt, func() {
		record("last")
		if ctx.Err() != context.Canceled {
			tt.Errorf("cleanup context not canceled")
		}
		if !Skipped(tt) {
			tt.Errorf("skip not visible during cleanup")
		}
		switch mode {
		case "cleanup-error":
			tt.Errorf("cleanup error")
		case "cleanup-fatal":
			tt.Fatal("cleanup fatal")
		case "cleanup-panic":
			panic("cleanup panic")
		}
	})
	defer record("defer")
	switch mode {
	case "prior-failure":
		tt.Fail()
	case "deferred-failure":
		defer tt.Errorf("deferred failure")
	}
	Skipf(tt, "setup unavailable: %s", "fixture")
	record("unreachable")
}

func TestHostedSkipLifecycle(t *testing.T) {
	for _, mode := range []string{"skip", "prior-failure", "deferred-failure", "cleanup-error", "cleanup-fatal", "cleanup-panic"} {
		t.Run(mode, func(t *testing.T) {
			var order []string
			result := runTest("skip", mode, func(tt TestingT) { exerciseSkip(tt, mode, func(s string) { order = append(order, s) }) })
			want := ResultSkipped
			if mode != "skip" {
				want = ResultFailed
			}
			if result.Result != want || result.SkipReason != "setup unavailable: fixture" {
				t.Fatalf("result: %+v", result)
			}
			if !reflect.DeepEqual(order, []string{"defer", "last", "first"}) {
				t.Fatalf("order %v", order)
			}
		})
	}
}

func TestNativeSkipLifecycle(t *testing.T) {
	if mode := os.Getenv("TESTY_SKIP_MODE"); mode != "" {
		exerciseSkip(newTWrapper(t, context.Background()), mode, func(s string) { t.Log("skip-marker:" + s) })
		return
	}
	for _, mode := range []string{"skip", "prior-failure", "deferred-failure", "cleanup-error", "cleanup-fatal", "cleanup-panic"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestNativeSkipLifecycle$", "-test.v")
			cmd.Env = append(os.Environ(), "TESTY_SKIP_MODE="+mode)
			output, err := cmd.CombinedOutput()
			if (err != nil) != (mode != "skip") {
				t.Fatalf("error %v: %s", err, output)
			}
			remaining := string(output)
			for _, marker := range []string{"defer", "last", "first"} {
				i := strings.Index(remaining, "skip-marker:"+marker)
				if i < 0 {
					t.Fatalf("missing marker %s: %s", marker, output)
				}
				remaining = remaining[i+len("skip-marker:"+marker):]
			}
			if mode == "skip" && !strings.Contains(string(output), "--- SKIP:") {
				t.Fatalf("not skipped: %s", output)
			}
		})
	}
}

func TestSkipAggregation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cases  []Tester
		result Result
		counts [4]int
	}{
		{"all-skipped", []Tester{func(tt TestingT) { Skipf(tt, "setup") }}, ResultIncomplete, [4]int{1, 0, 0, 1}},
		{"mixed", []Tester{func(TestingT) {}, func(tt TestingT) { Skipf(tt, "setup") }}, ResultIncomplete, [4]int{2, 1, 0, 1}},
		{"failure", []Tester{func(tt TestingT) { tt.Fail() }, func(tt TestingT) { Skipf(tt, "setup") }}, ResultFailed, [4]int{2, 0, 1, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
			for i, tester := range tc.cases {
				name := string(rune('a' + i))
				pkg.tests[name] = testCase{Name: name, tester: tester}
			}
			result := BuildSuiteResult(time.Now(), []TestResult{runPackage("skip", pkg)})
			if result.Result != tc.result || result.Subtests[0].Result != tc.result {
				t.Fatalf("result %+v", result)
			}
			total, passed, failed, skipped := result.SumTestStatsWithSkipped()
			if got := [4]int{total, passed, failed, skipped}; got != tc.counts {
				t.Fatalf("counts %v want %v", got, tc.counts)
			}
			a, b, c := result.SumTestStats()
			if [3]int{a, b, c} != [3]int{total, passed, failed} {
				t.Fatal("legacy stats disagree")
			}
			db := &InMemoryDB{}
			if _, err := db.Save(context.Background(), result); err != nil {
				t.Fatal(err)
			}
			summaries, _, err := db.Enumerate(context.Background(), 1)
			if err != nil || summaries[0].Skipped != skipped {
				t.Fatalf("summaries %+v err %v", summaries, err)
			}
		})
	}
}

func TestSkipNestedResult(t *testing.T) {
	result := runTest("skip", "parent", func(tt TestingT) {
		if !tt.Run("child", func(child TestingT) { Skipf(child, "missing fixture") }) {
			tt.Errorf("skip is not failure")
		}
	})
	if result.Result != ResultIncomplete {
		t.Fatalf("result %+v", result)
	}
	parentSkip := runTest("skip", "parent", func(tt TestingT) { tt.Run("child", func(TestingT) {}); Skipf(tt, "remaining setup") })
	if parentSkip.Result != ResultIncomplete {
		t.Fatalf("parent not incomplete: %+v", parentSkip)
	}
	total, passed, failed, skipped := parentSkip.SumTestStatsWithSkipped()
	if [4]int{total, passed, failed, skipped} != [4]int{2, 1, 0, 1} {
		t.Fatalf("parent skip not counted: %+v", parentSkip)
	}
}

func TestSkipCannotMaskPanic(t *testing.T) {
	for _, bodyPanic := range []bool{true, false} {
		result := runTest("skip", "panic", func(tt TestingT) {
			Cleanup(tt, func() { Skipf(tt, "cleanup skip") })
			if bodyPanic {
				panic("body panic")
			}
			Cleanup(tt, func() { panic("cleanup panic") })
		})
		if result.Result != ResultFailed {
			t.Fatalf("panic masked: %+v", result)
		}
	}
}

func TestSkipUnsupported(t *testing.T) {
	for _, f := range []func(TestingT){func(tt TestingT) { Skipf(tt, "unsupported") }, func(tt TestingT) { Skipped(tt) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("unsupported must panic")
				}
			}()
			f(&tHelper{})
		}()
	}
}

func TestNativeSkipCannotMaskCleanupPanic(t *testing.T) {
	if os.Getenv("TESTY_SKIP_PANIC") != "" {
		tt := newTWrapper(t, context.Background())
		Cleanup(tt, func() { Skipf(tt, "cleanup skip") })
		Cleanup(tt, func() { panic("real cleanup defect") })
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestNativeSkipCannotMaskCleanupPanic$", "-test.v")
	cmd.Env = append(os.Environ(), "TESTY_SKIP_PANIC=1")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "real cleanup defect") {
		t.Fatalf("panic masked: err=%v output=%s", err, output)
	}
}

func TestNativeSkipCannotMaskBodyPanic(t *testing.T) {
	if mode := os.Getenv("TESTY_BODY_PANIC"); mode != "" {
		body := func(tt TestingT) {
			Cleanup(tt, func() { Skipf(tt, "cleanup skip") })
			panic("real body defect")
		}
		pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
		pkg.tests["case"] = testCase{Name: "case", tester: func(tt TestingT) {
			if mode == "nested" {
				tt.Run("child", body)
			} else {
				body(tt)
			}
		}}
		instance = testy{tests: orderedmap.OrderedMap[string, *testPkg]{"skip": pkg}}
		RunAsTest(t)
		return
	}
	for _, mode := range []string{"case", "nested"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestNativeSkipCannotMaskBodyPanic$", "-test.v")
		cmd.Env = append(os.Environ(), "TESTY_BODY_PANIC="+mode)
		output, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(output), "real body defect") {
			t.Fatalf("panic masked mode=%s err=%v output=%s", mode, err, output)
		}
	}
}

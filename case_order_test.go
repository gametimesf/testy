package testy

import (
	"context"
	"reflect"
	"testing"

	"github.com/gametimesf/testy/internal/orderedmap"
)

func TestCaseDispatchOrderPreservesEveryCanonicalResult(t *testing.T) {
	for _, order := range [][]string{{"c", "a", "b"}, {"c", "c"}, {"c", "c", "a"}, {"unknown", "b", "a"}, nil} {
		var executed []string
		pkg := &testPkg{tests: orderedmap.OrderedMap[string, testCase]{}}
		for _, name := range []string{"a", "b", "c"} {
			name := name
			pkg.tests[name] = testCase{Name: name, Concurrent: true, tester: func(tt TestingT) {
				executed = append(executed, name)
				tt.Run("one", func(TestingT) {})
				tt.Run("two", func(TestingT) {})
			}}
		}
		result := runPackageWithOptions(context.Background(), "pkg", pkg, RunOptions{CaseExecutor: NewCaseExecutor(1), CaseOrder: map[string][]string{"pkg": order}})
		want := []string{"a", "b", "c"}
		if len(order) == 3 && order[0] == "c" && order[1] == "a" {
			want = order
		}
		if !reflect.DeepEqual(executed, want) {
			t.Fatalf("order=%v executed=%v want=%v", order, executed, want)
		}
		if len(order) > 0 && !reflect.DeepEqual(order, want) && len(result.Msgs) == 0 {
			t.Fatal("invalid scheduling hint was not diagnosed")
		}
		for i, name := range []string{"a", "b", "c"} {
			if result.Subtests[i].Name != name || result.Subtests[i].Result != ResultPassed || result.Subtests[i].Subtests[0].Name != name+"/one" || result.Subtests[i].Subtests[1].Name != name+"/two" {
				t.Fatalf("canonical identity/steps changed %+v", result)
			}
		}
	}
}

package testy

import (
	"fmt"
)

// TestEach runs tester as a subtest for each value in values. The values should have a good default string
// representation so the subtest names are legible. (Consider implementing fmt.Stringer for complex structs.)
func TestEach[V any](t TestingT, values []V, tester func(TestingT, V)) {
	for _, v := range values {
		t.Run(fmt.Sprintf("%v", v), func(t TestingT) {
			tester(t, v)
		})
	}
}

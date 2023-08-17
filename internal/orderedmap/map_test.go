package orderedmap

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ExampleOrderedMap() {
	m := OrderedMap[int, string]{}
	m[2] = "two"
	m[3] = "three"
	m[1] = "one"
	m[5] = "five"
	m[0] = "zero"
	m[4] = "four"

	m.Iterate(func(k int, v string) bool {
		fmt.Println(v)
		return k != 4
	})

	// Output:
	// zero
	// one
	// two
	// three
	// four
}

func TestNilIterate(t *testing.T) {
	var m OrderedMap[int, any]
	assert.Nil(t, m)
	// only testing that this does not panic
	m.Iterate(func(k int, v any) bool {
		assert.FailNow(t, "Should not iterate a nil map")
		return false // unreachable but the compiler does not know that
	})
}

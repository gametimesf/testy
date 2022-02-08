package orderedmap

import (
	"testing"
)

func TestOrderedMap(t *testing.T) {
	m := OrderedMap[int, string]{}
	m[2] = "two"
	m[3] = "three"
	m[1] = "one"
	m[5] = "five"
	m[0] = "zero"
	m[4] = "four"

	i := 0
	pass := true
	m.Iterate(func(k int, v string) bool {
		if k != i {
			t.Errorf("Expected %d, got %d", i, k)
			pass = false
			return false
		}
		i++
		return true
	})
	if !pass {
		t.Failed()
	}
}

package orderedmap

import (
	"fmt"
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

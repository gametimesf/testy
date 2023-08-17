package tests

import (
	"github.com/stretchr/testify/assert"

	. "github.com/gametimesf/testy"
	"github.com/gametimesf/testy/example/fib"
)

var _ = Test("70th Fibonacci number", func(t TestingT) {
	f := fib.Fib(70)
	assert.Equal(t, 190_392_490_709_135, f)
})

var _ = Test("Fibonacci number", func(t TestingT) {
	expected := []int{
		0, 1, 1, 2, 3, 5, 8, 13, 21, 34, 55,
	}
	TestEach(t, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, func(t TestingT, i int) {
		f := fib.Fib(i)
		assert.Equal(t, expected[i], f)
	})
})

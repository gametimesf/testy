package testy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEach(t *testing.T) {
	var run []string
	Each(tWrapper{t: t}, []string{"a", "b", "c"}, func(t TestingT, s string) {
		run = append(run, s)
	})

	if assert.Len(t, run, 3) {
		assert.Equal(t, "a", run[0])
		assert.Equal(t, "b", run[1])
		assert.Equal(t, "c", run[2])
	}
}

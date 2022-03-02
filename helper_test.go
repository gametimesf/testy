package testy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestEach(t *testing.T) {
	var run []string
	TestEach(tWrapper{t: t}, []string{"a", "b", "c"}, func(t TestingT, s string) {
		run = append(run, s)
	})

	require.Len(t, run, 3)
	assert.Equal(t, "a", run[0])
	assert.Equal(t, "b", run[1])
	assert.Equal(t, "c", run[2])
}

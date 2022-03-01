package testy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testResultTestData = TestResult{
	Name:   "root",
	Result: ResultFailed,
	Subtests: []TestResult{
		{
			Name:   "tree 1",
			Result: ResultFailed,
			Subtests: []TestResult{
				{
					Name:   "tree 1 leaf 1",
					Result: ResultFailed,
				},
				{
					Name:   "tree 1 leaf 2",
					Result: ResultPassed,
				},
			},
		},
		{
			Name:   "tree 2",
			Result: ResultFailed,
			Subtests: []TestResult{
				{
					Name:   "tree 2 intermediate 1",
					Result: ResultFailed,
					Subtests: []TestResult{
						{
							Name:   "tree 2 intermediate 1 leaf 1",
							Result: ResultFailed,
						},
						{
							Name:   "tree 2 intermediate 1 leaf 2",
							Result: ResultFailed,
						},
					},
				},
				{
					Name:   "tree 1 intermediate 2",
					Result: ResultPassed,
					Subtests: []TestResult{
						{
							Name:   "tree 2 intermediate 2 leaf 1",
							Result: ResultPassed,
						},
						{
							Name:   "tree 2 intermediate 2 leaf 2",
							Result: ResultPassed,
						},
					},
				},
			},
		},
		{
			Name:   "tree 3",
			Result: ResultPassed,
		},
		{
			Name:   "tree 4",
			Result: ResultFailed,
		},
		{
			Name:   "tree 5",
			Result: ResultPassed,
			Subtests: []TestResult{
				{
					Name:   "tree 5 leaf 1",
					Result: ResultPassed,
				},
				{
					Name:   "tree 5 leaf 2",
					Result: ResultPassed,
				},
			},
		},
	},
}

func TestSumTestStats(t *testing.T) {
	t.Run("full tree", func(t *testing.T) {
		total, passed, failed := testResultTestData.SumTestStats()
		assert.Equal(t, total, 10)
		assert.Equal(t, passed, 6)
		assert.Equal(t, failed, 4)
	})

	t.Run("tree 2", func(t *testing.T) {
		total, passed, failed := testResultTestData.Subtests[1].SumTestStats()
		assert.Equal(t, total, 4)
		assert.Equal(t, passed, 2)
		assert.Equal(t, failed, 2)
	})

	t.Run("tree 3", func(t *testing.T) {
		total, passed, failed := testResultTestData.Subtests[2].SumTestStats()
		assert.Equal(t, total, 1)
		assert.Equal(t, passed, 1)
		assert.Equal(t, failed, 0)
	})

	t.Run("tree 4", func(t *testing.T) {
		total, passed, failed := testResultTestData.Subtests[3].SumTestStats()
		assert.Equal(t, total, 1)
		assert.Equal(t, passed, 0)
		assert.Equal(t, failed, 1)
	})
}

func TestFindFailingTests(t *testing.T) {
	t.Run("full tree", func(t *testing.T) {
		failed := testResultTestData.FindFailingTests()
		require.Len(t, failed, 3)
		found := make([]string, 0, 3)
		for _, t := range failed {
			found = append(found, t.Name)
		}
		assert.ElementsMatch(t, []string{"tree 1 leaf 1", "tree 2 intermediate 1", "tree 4"}, found)
	})

	t.Run("tree 3", func(t *testing.T) {
		failed := testResultTestData.Subtests[2].FindFailingTests()
		assert.Len(t, failed, 0)
	})

	t.Run("tree 4", func(t *testing.T) {
		failed := testResultTestData.Subtests[3].FindFailingTests()
		require.Len(t, failed, 1)
		assert.Equal(t, "tree 4", failed[0].Name)
	})

	t.Run("tree 5", func(t *testing.T) {
		failed := testResultTestData.Subtests[4].FindFailingTests()
		assert.Len(t, failed, 0)
	})

	t.Run("tree 2 intermediate 1", func(t *testing.T) {
		failed := testResultTestData.Subtests[1].Subtests[0].FindFailingTests()
		require.Len(t, failed, 1)
		assert.Equal(t, "tree 2 intermediate 1", failed[0].Name)
	})
}

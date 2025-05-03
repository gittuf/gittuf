// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package set

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testItems = []struct {
	input                  []int
	expectedSortedContents []int
	marshalJSON            string
}{
	{
		input:                  nil,
		expectedSortedContents: []int{},
		marshalJSON:            "[]",
	},
	{
		input:                  []int{},
		expectedSortedContents: []int{},
		marshalJSON:            "[]",
	},
	{
		input:                  []int{1, 2, 3},
		expectedSortedContents: []int{1, 2, 3},
		marshalJSON:            "[1,2,3]",
	},
	{
		input:                  []int{1, 1, 2, 3},
		expectedSortedContents: []int{1, 2, 3},
		marshalJSON:            "[1,2,3]",
	},
	{
		input:                  []int{3, 1, 3, 2, 3},
		expectedSortedContents: []int{1, 2, 3},
		marshalJSON:            "[1,2,3]",
	},
	{
		input:                  []int{4},
		expectedSortedContents: []int{4},
		marshalJSON:            "[4]",
	},
}

func TestNewSet(t *testing.T) {
	set := NewSet[int]()
	setHasOnlyTheItems(t, set)
}

func TestNewSetFromItems(t *testing.T) {
	for _, tt := range testItems {
		set := NewSetFromItems(tt.input...)
		setHasOnlyTheItems(t, set, tt.expectedSortedContents...)
	}
}

func TestMarshalJSON(t *testing.T) {
	t.Run("valid set", func(t *testing.T) {
		for _, tt := range testItems {
			set := NewSetFromItems(tt.input...)
			jsonBytes, err := set.MarshalJSON()
			require.NoError(t, err)
			require.Equal(t, tt.marshalJSON, string(jsonBytes))
		}
	})

	t.Run("nil set contents", func(t *testing.T) {
		set := &Set[int]{}

		jsonBytes, err := set.MarshalJSON()
		require.NoError(t, err)
		require.Equal(t, "null", string(jsonBytes))
	})
}

func TestUnmarshalJSON(t *testing.T) {
	t.Run("valid json collections", func(t *testing.T) {
		for _, tt := range []struct {
			json     string
			expected []int
		}{
			{
				json:     "[]",
				expected: []int{},
			},
			{
				json:     "[1,2,3]",
				expected: []int{1, 2, 3},
			},
			{
				json:     "[1,1,2,3]",
				expected: []int{1, 2, 3},
			},
			{
				json:     "[3,1,3,2,3]",
				expected: []int{1, 2, 3},
			},
			{
				json:     "[1 ,	2,   3]",
				expected: []int{1, 2, 3},
			},
			{
				json:     "[4]",
				expected: []int{4},
			},
		} {
			set := NewSet[int]()
			err := set.UnmarshalJSON([]byte(tt.json))
			require.NoError(t, err)
			setHasOnlyTheItems(t, set, tt.expected...)
		}
	})

	t.Run("invalid json collections", func(t *testing.T) {
		testInvalidJSONs := []string{"", "0", "[1, 2", "(1, 2, 3)", "1, 2"}
		for _, json := range testInvalidJSONs {
			set := NewSet[int]()
			err := set.UnmarshalJSON([]byte(json))
			require.Error(t, err)
		}
	})

	t.Run("overwrite existing set", func(t *testing.T) {
		set := NewSet[int]()

		err := set.UnmarshalJSON([]byte("[1, 2, 3]"))
		require.NoError(t, err)
		setHasOnlyTheItems(t, set, 1, 2, 3)

		err = set.UnmarshalJSON([]byte("[-1,-2,-3]"))
		require.NoError(t, err)
		setHasOnlyTheItems(t, set, -1, -2, -3)

		err = set.UnmarshalJSON([]byte("[]"))
		require.NoError(t, err)
		setHasOnlyTheItems(t, set)
	})
}

func TestHas(t *testing.T) {
	t.Run("populated set", func(t *testing.T) {
		set := NewSetFromItems(1, 2, 3)

		assert.False(t, set.Has(0))
		assert.True(t, set.Has(1))
		assert.True(t, set.Has(2))
		assert.True(t, set.Has(3))

		// check that set is not mutated
		setMarshalJSONIs(t, set, "[1,2,3]")
	})

	t.Run("set with nil contents", func(t *testing.T) {
		set := &Set[int]{}

		assert.False(t, set.Has(0))
		assert.False(t, set.Has(1))

		// check that set is not mutated
		setMarshalJSONIs(t, set, "null")
	})
}

func TestContents(t *testing.T) {
	t.Run("constructed set", func(t *testing.T) {
		for _, tt := range testItems {
			set := NewSetFromItems(tt.input...)
			c := set.Contents()
			slices.Sort(c)
			assert.Equal(t, tt.expectedSortedContents, c)
		}
	})

	t.Run("set with nil contents", func(t *testing.T) {
		set := &Set[int]{}
		assert.Nil(t, set.Contents())
	})
}

func TestAdd(t *testing.T) {
	t.Run("constructed set", func(t *testing.T) {
		set := NewSet[int]()
		setMarshalJSONIs(t, set, "[]")

		set.Add(0)
		setMarshalJSONIs(t, set, "[0]")

		set.Add(1)
		setMarshalJSONIs(t, set, "[0,1]")

		set.Add(1)
		setMarshalJSONIs(t, set, "[0,1]")

		set.Add(2)
		setMarshalJSONIs(t, set, "[0,1,2]")
	})

	t.Run("set with nil contents", func(t *testing.T) {
		set := &Set[int]{}
		setMarshalJSONIs(t, set, "null")

		set.Add(0)
		setMarshalJSONIs(t, set, "[0]")
	})
}

func TestRemove(t *testing.T) {
	t.Run("constructed set", func(t *testing.T) {
		set := NewSetFromItems(0, 1, 2)
		setMarshalJSONIs(t, set, "[0,1,2]")

		set.Remove(4)
		setMarshalJSONIs(t, set, "[0,1,2]")

		set.Remove(0)
		setMarshalJSONIs(t, set, "[1,2]")

		set.Remove(0)
		setMarshalJSONIs(t, set, "[1,2]")

		set.Remove(2)
		setMarshalJSONIs(t, set, "[1]")

		set.Remove(1)
		setMarshalJSONIs(t, set, "[]")

		set.Remove(0)
		setMarshalJSONIs(t, set, "[]")
	})

	t.Run("set with nil contents", func(t *testing.T) {
		set := &Set[int]{}
		setMarshalJSONIs(t, set, "null")

		set.Remove(0)
		setMarshalJSONIs(t, set, "null")
	})
}

func TestExtend(t *testing.T) {
	set := NewSetFromItems(0)
	setMarshalJSONIs(t, set, "[0]")

	set.Extend(NewSetFromItems(1, 2))
	setMarshalJSONIs(t, set, "[0,1,2]")

	set.Extend(NewSetFromItems(0, 1, 2))
	setMarshalJSONIs(t, set, "[0,1,2]")

	set.Extend(set)
	setMarshalJSONIs(t, set, "[0,1,2]")

	set.Extend(&Set[int]{})
	setMarshalJSONIs(t, set, "[0,1,2]")

	set.Extend(nil)
	setMarshalJSONIs(t, set, "[0,1,2]")
}

func TestIntersection(t *testing.T) {
	emptySet := NewSet[int]()
	bigSet := NewSetFromItems(0, 1, 2, 3, 4, 5)

	for _, tt := range []struct {
		set            *Set[int]
		expectedSetStr string
	}{
		{
			set:            emptySet,
			expectedSetStr: "[]",
		},
		{
			set:            bigSet,
			expectedSetStr: "[0,1,2,3,4,5]",
		},

		{
			set:            emptySet.Intersection(emptySet),
			expectedSetStr: "[]",
		},
		{
			set:            bigSet.Intersection(bigSet),
			expectedSetStr: "[0,1,2,3,4,5]",
		},
		{
			set:            emptySet.Intersection(bigSet),
			expectedSetStr: "[]",
		},
		{
			set:            bigSet.Intersection(emptySet),
			expectedSetStr: "[]",
		},

		{
			set:            bigSet.Intersection(NewSetFromItems(3, 4, 5, 6, 7, 8)),
			expectedSetStr: "[3,4,5]",
		},

		{
			set:            bigSet.Intersection(nil),
			expectedSetStr: "[]",
		},
	} {
		setMarshalJSONIs(t, tt.set, tt.expectedSetStr)
	}
}

func TestMinus(t *testing.T) {
	emptySet := NewSet[int]()
	bigSet := NewSetFromItems(0, 1, 2, 3, 4, 5)

	for _, tt := range []struct {
		set            *Set[int]
		expectedSetStr string
	}{
		{
			set:            emptySet,
			expectedSetStr: "[]",
		},
		{
			set:            bigSet,
			expectedSetStr: "[0,1,2,3,4,5]",
		},
		{
			set:            emptySet.Minus(emptySet),
			expectedSetStr: "[]",
		},
		{
			set:            bigSet.Minus(bigSet),
			expectedSetStr: "[]",
		},
		{
			set:            emptySet.Minus(bigSet),
			expectedSetStr: "[]",
		},
		{
			set:            bigSet.Minus(emptySet),
			expectedSetStr: "[0,1,2,3,4,5]",
		},
		{
			set:            bigSet.Minus(NewSetFromItems(3, 4, 5, 6, 7, 8)),
			expectedSetStr: "[0,1,2]",
		},
		{
			set:            bigSet.Minus(NewSetFromItems(6, 7, 8)),
			expectedSetStr: "[0,1,2,3,4,5]",
		},
		{
			set:            bigSet.Minus(nil),
			expectedSetStr: "[0,1,2,3,4,5]",
		},
	} {
		setMarshalJSONIs(t, tt.set, tt.expectedSetStr)
	}
}

func TestEqual(t *testing.T) {
	emptySet := NewSet[int]()
	bigSet := NewSetFromItems(0, 1, 2, 3, 4, 5)

	for _, tt := range []struct {
		expectedTrue bool
		errorMsg     string
	}{
		{
			expectedTrue: !emptySet.Equal(bigSet),
			errorMsg:     "empty and populated set are not equal",
		},
		{
			expectedTrue: !bigSet.Equal(emptySet),
			errorMsg:     "populated and empty set are not equal",
		},
		{
			expectedTrue: emptySet.Equal(emptySet), // nolint:gocritic
			errorMsg:     "empty set is equal to itself",
		},
		{
			expectedTrue: bigSet.Equal(bigSet), // nolint:gocritic
			errorMsg:     "populated set is equal to itself",
		},
		{
			expectedTrue: emptySet.Equal(NewSet[int]()),
			errorMsg:     "sets with same contents should be equal",
		},
		{
			expectedTrue: !emptySet.Equal(NewSetFromItems(0)),
			errorMsg:     "sets with additional contents should be equal",
		},
		{
			expectedTrue: bigSet.Equal(NewSetFromItems(5, 4, 3, 2, 1, 0)),
			errorMsg:     "sets with same contents should be equal",
		},
		{
			expectedTrue: !bigSet.Equal(NewSetFromItems(4, 3, 2, 1)),
			errorMsg:     "sets missing content should not be equal",
		},
		{
			expectedTrue: !bigSet.Equal(NewSetFromItems(0, 1, 2, -3, 4, 5)),
			errorMsg:     "sets with different contents should be equal",
		},
		{
			expectedTrue: !emptySet.Equal(nil),
			errorMsg:     "empty set is not equal to nil",
		},
	} {
		assert.True(t, tt.expectedTrue, tt.errorMsg)
	}
}

// check that set has only the items, function assumes that items does not have duplicates
func setHasOnlyTheItems(t *testing.T, set *Set[int], items ...int) {
	t.Helper()
	require.Len(t, items, set.Len())
	for _, i := range items {
		if !set.Has(i) {
			t.Errorf("set is missing item %d", i)
		}
	}
}

func setMarshalJSONIs(t *testing.T, set *Set[int], setStr string) {
	t.Helper()
	jsonBytes, err := set.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, setStr, string(jsonBytes))
}

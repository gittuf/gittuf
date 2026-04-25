// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPredicateToPBStruct(t *testing.T) {
	t.Run("converts map predicate", func(t *testing.T) {
		predicate := map[string]any{
			"repo":      "gittuf",
			"threshold": 2,
			"active":    true,
			"scopes":    []any{"push", "pull"},
			"metadata": map[string]any{
				"team": "security",
			},
		}

		predicateStruct, err := PredicateToPBStruct(predicate)
		require.Nil(t, err)

		expected := map[string]any{
			"repo":      "gittuf",
			"threshold": float64(2),
			"active":    true,
			"scopes":    []any{"push", "pull"},
			"metadata": map[string]any{
				"team": "security",
			},
		}

		assert.Equal(t, expected, predicateStruct.AsMap())
	})

	t.Run("converts struct predicate", func(t *testing.T) {
		type testPredicate struct {
			Name     string `json:"name"`
			Attempts int    `json:"attempts"`
			Approved bool   `json:"approved"`
			Reviewer string `json:"reviewer"`
		}

		predicateStruct, err := PredicateToPBStruct(testPredicate{
			Name:     "review",
			Attempts: 3,
			Approved: true,
			Reviewer: "alice",
		})
		require.Nil(t, err)

		expected := map[string]any{
			"name":     "review",
			"attempts": float64(3),
			"approved": true,
			"reviewer": "alice",
		}

		assert.Equal(t, expected, predicateStruct.AsMap())
	})

	t.Run("returns marshal error", func(t *testing.T) {
		predicate := map[string]any{
			"invalid": func() {},
		}

		predicateStruct, err := PredicateToPBStruct(predicate)
		assert.ErrorContains(t, err, "unsupported type")
		assert.Nil(t, predicateStruct)
	})

	t.Run("returns unmarshal error for non-object JSON", func(t *testing.T) {
		predicate := []string{"not", "an", "object"}

		predicateStruct, err := PredicateToPBStruct(predicate)
		assert.ErrorContains(t, err, "cannot unmarshal array")
		assert.Nil(t, predicateStruct)
	})
}

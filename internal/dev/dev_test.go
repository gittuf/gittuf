// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dev

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInDevMode(t *testing.T) {
	t.Run("dev mode unset", func(t *testing.T) {
		t.Setenv(DevModeKey, "")
		assert.False(t, InDevMode())
	})

	t.Run("dev mode set to 0", func(t *testing.T) {
		t.Setenv(DevModeKey, "0")
		assert.False(t, InDevMode())
	})

	t.Run("dev mode set to 1", func(t *testing.T) {
		t.Setenv(DevModeKey, "1")
		assert.True(t, InDevMode())
	})
}

func TestErrNotInDevMode(t *testing.T) {
	assert.Error(t, ErrNotInDevMode)
	assert.Contains(t, ErrNotInDevMode.Error(), "this feature is only available in developer mode")
	assert.Contains(t, ErrNotInDevMode.Error(), DevModeKey)
}

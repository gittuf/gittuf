// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cmd := New()
	assert.Equal(t, "cache", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.True(t, cmd.HasSubCommands())
}

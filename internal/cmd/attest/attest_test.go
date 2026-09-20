// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cmd := New()
	assert.NotNil(t, cmd)
	assert.Equal(t, "attest", cmd.Use)
	assert.True(t, cmd.HasSubCommands())
}

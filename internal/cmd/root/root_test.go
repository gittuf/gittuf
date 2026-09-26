// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cmd := New()
	assert.NotNil(t, cmd)
	assert.Equal(t, "gittuf", cmd.Use)

	// Check if all subcommands are added
	assert.True(t, cmd.HasSubCommands())

	// Check flags
	assert.NotNil(t, cmd.PersistentFlags().Lookup("no-color"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("verbose"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("profile"))
}

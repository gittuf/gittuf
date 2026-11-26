// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attest

import (
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cmd := New()
	assert.NotNil(t, cmd)
	assert.Equal(t, "attest", cmd.Use)
	assert.True(t, cmd.HasSubCommands())
}

func TestSubcommandRequired(t *testing.T) {
	t.Run("unknown subcommand", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New(), "nosuch")
		assert.ErrorContains(t, err, `unknown command "nosuch" for "attest"`)
	})

	t.Run("no subcommand prints help", func(t *testing.T) {
		_, stdOut, _, err := cmd.ExecuteCommandC(New(), []string{}...)
		assert.NoError(t, err)
		assert.Contains(t, stdOut.String(), "attest [command]")
	})
}

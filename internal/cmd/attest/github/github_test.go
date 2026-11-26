// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	options := &persistent.Options{}
	cmd := New(options)
	assert.Equal(t, "github", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.True(t, cmd.HasSubCommands())
}

func TestSubcommandRequired(t *testing.T) {
	t.Run("unknown subcommand", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New(&persistent.Options{}), "nosuch")
		assert.ErrorContains(t, err, `unknown command "nosuch" for "github"`)
	})

	t.Run("no subcommand prints help", func(t *testing.T) {
		_, stdOut, _, err := cmd.ExecuteCommandC(New(&persistent.Options{}), []string{}...)
		assert.NoError(t, err)
		assert.Contains(t, stdOut.String(), "github [command]")
	})
}

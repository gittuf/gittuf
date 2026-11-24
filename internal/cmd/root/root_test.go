// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectStorerBackend(t *testing.T) {
	t.Run("rejects an unknown backend", func(t *testing.T) {
		t.Setenv(gittuf.StorerBackendEnvKey, "libgit2")

		assert.ErrorIs(t, selectStorerBackend(), gittuf.ErrUnknownStorerBackend)
	})

	t.Run("an unset environment selects the git binary backend", func(t *testing.T) {
		t.Setenv(gittuf.StorerBackendEnvKey, "")

		require.Nil(t, selectStorerBackend())
		assert.Equal(t, gittuf.StorerBackendBinary, gittuf.StorerBackendInUse())
	})

	t.Run("the go-git backend requires developer mode", func(t *testing.T) {
		t.Setenv(gittuf.StorerBackendEnvKey, string(gittuf.StorerBackendGoGit))
		t.Setenv(dev.DevModeKey, "")

		assert.ErrorIs(t, selectStorerBackend(), dev.ErrNotInDevMode)
	})

	t.Run("the go-git backend applies in developer mode", func(t *testing.T) {
		t.Setenv(gittuf.StorerBackendEnvKey, string(gittuf.StorerBackendGoGit))
		t.Setenv(dev.DevModeKey, "1")

		require.Nil(t, selectStorerBackend())
		assert.Equal(t, gittuf.StorerBackendGoGit, gittuf.StorerBackendInUse())
	})
}

func TestNoStorerFlag(t *testing.T) {
	assert.Nil(t, New().PersistentFlags().Lookup("storer"), "the backend is selected by "+gittuf.StorerBackendEnvKey+" alone")
}

func TestStorerTraceFlagsAreRegistered(t *testing.T) {
	flags := New().PersistentFlags()

	trace := flags.Lookup("storer-trace")
	require.NotNil(t, trace)
	assert.Equal(t, "false", trace.DefValue)

	file := flags.Lookup("storer-trace-file")
	require.NotNil(t, file)
	assert.Equal(t, "storer.trace", file.DefValue)
}

func TestReportStorerTraceIsSilentWhenDisabled(t *testing.T) {
	t.Setenv(gittuf.StorerBackendEnvKey, "")

	o := &options{storerTrace: false, storerTraceFile: "storer.trace"}
	require.Nil(t, o.PreRunE(nil, nil))

	out := &bytes.Buffer{}
	ReportStorerTrace(out)
	assert.Empty(t, out.String())
}

func TestNew(t *testing.T) {
	cmd := New()
	assert.NotNil(t, cmd)
	assert.Equal(t, "gittuf", cmd.Use)

	// Check if subcommands are added
	assert.True(t, cmd.HasSubCommands())

	// Check flags
	assert.NotNil(t, cmd.PersistentFlags().Lookup("no-color"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("verbose"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("profile"))
}

func TestUnknownSubcommand(t *testing.T) {
	tests := map[string]struct {
		args        []string
		expectedErr string
	}{
		"top level": {
			args:        []string{"aplly"},
			expectedErr: `unknown command "aplly" for "gittuf"`,
		},
		"policy": {
			args:        []string{"policy", "aplly"},
			expectedErr: `unknown command "aplly" for "gittuf policy"`,
		},
		"nested": {
			args:        []string{"rsl", "remote", "chek"},
			expectedErr: `unknown command "chek" for "gittuf rsl remote"`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, _, err := cmd.ExecuteCommandC(New(), test.args...)
			assert.ErrorContains(t, err, test.expectedErr)
		})
	}

	t.Run("no subcommand prints help", func(t *testing.T) {
		_, stdOut, _, err := cmd.ExecuteCommandC(New(), "policy")
		assert.NoError(t, err)
		assert.Contains(t, stdOut.String(), "gittuf policy [command]")
	})

	t.Run("every command with subcommands", func(t *testing.T) {
		var check func(c *cobra.Command)
		check = func(c *cobra.Command) {
			for _, subcommand := range c.Commands() {
				if subcommand.HasSubCommands() {
					args := append(strings.Fields(strings.TrimPrefix(subcommand.CommandPath(), "gittuf")), "nosuch")
					_, _, _, err := cmd.ExecuteCommandC(New(), args...)
					assert.ErrorContains(t, err, `unknown command "nosuch"`, subcommand.CommandPath())
				}
				check(subcommand)
			}
		}
		check(New())
	})
}

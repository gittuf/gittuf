// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"bytes"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreRunEStorerBackend(t *testing.T) {
	t.Run("rejects an unknown backend", func(t *testing.T) {
		o := &options{storer: "libgit2"}

		err := o.PreRunE(nil, nil)
		assert.ErrorIs(t, err, gittuf.ErrUnknownStorerBackend)
	})

	t.Run("accepts the git binary backend", func(t *testing.T) {
		o := &options{storer: string(gittuf.StorerBackendBinary)}

		require.Nil(t, o.PreRunE(nil, nil))
	})

	t.Run("accepts the go-git backend", func(t *testing.T) {
		o := &options{storer: string(gittuf.StorerBackendGoGit)}

		require.Nil(t, o.PreRunE(nil, nil))
	})
}

func TestStorerFlagIsRegistered(t *testing.T) {
	flag := New().PersistentFlags().Lookup("storer")

	require.NotNil(t, flag)
	assert.Equal(t, string(gittuf.StorerBackendBinary), flag.DefValue)
	assert.Contains(t, flag.Usage, "experimental")
}

func TestStorerTraceFlagIsRegistered(t *testing.T) {
	flag := New().PersistentFlags().Lookup("storer-trace")

	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
}

func TestReportStorerTraceIsSilentWhenDisabled(t *testing.T) {
	o := &options{storer: string(gittuf.StorerBackendBinary), storerTrace: false}
	require.Nil(t, o.PreRunE(nil, nil))

	out := &bytes.Buffer{}
	ReportStorerTrace(out)
	assert.Empty(t, out.String())
}

func TestPreRunEStorerBackendPrecedence(t *testing.T) {
	newCmd := func(o *options) *cobra.Command {
		cmd := &cobra.Command{Use: "test"}
		o.AddFlags(cmd)
		return cmd
	}

	t.Run("the environment applies when the flag was not passed", func(t *testing.T) {
		t.Setenv(gittuf.StorerBackendEnvKey, "go-git")

		o := &options{}
		cmd := newCmd(o)
		require.Nil(t, o.PreRunE(cmd, nil))

		assert.Equal(t, gittuf.StorerBackendGoGit, gittuf.StorerBackendInUse())
	})

	t.Run("an explicitly passed flag beats the environment", func(t *testing.T) {
		t.Setenv(gittuf.StorerBackendEnvKey, "go-git")

		o := &options{}
		cmd := newCmd(o)
		require.Nil(t, cmd.PersistentFlags().Set("storer", "binary"))
		require.Nil(t, o.PreRunE(cmd, nil))

		assert.Equal(t, gittuf.StorerBackendBinary, gittuf.StorerBackendInUse())
	})

	t.Run("the flag applies when the environment is unset", func(t *testing.T) {
		t.Setenv(gittuf.StorerBackendEnvKey, "")

		o := &options{}
		cmd := newCmd(o)
		require.Nil(t, cmd.PersistentFlags().Set("storer", "go-git"))
		require.Nil(t, o.PreRunE(cmd, nil))

		assert.Equal(t, gittuf.StorerBackendGoGit, gittuf.StorerBackendInUse())
	})

	t.Run("an unparseable environment value falls back to the flag", func(t *testing.T) {
		t.Setenv(gittuf.StorerBackendEnvKey, "libgit2")

		o := &options{}
		cmd := newCmd(o)
		require.Nil(t, o.PreRunE(cmd, nil))

		assert.Equal(t, gittuf.StorerBackendBinary, gittuf.StorerBackendInUse())
	})

	t.Run("an unparseable flag is an error", func(t *testing.T) {
		t.Setenv(gittuf.StorerBackendEnvKey, "")

		o := &options{}
		cmd := newCmd(o)
		require.Nil(t, cmd.PersistentFlags().Set("storer", "libgit2"))

		assert.ErrorIs(t, o.PreRunE(cmd, nil), gittuf.ErrUnknownStorerBackend)
	})
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"bytes"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/dev"
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

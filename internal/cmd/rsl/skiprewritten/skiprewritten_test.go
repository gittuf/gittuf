// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package skiprewritten

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkipRewritten(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		_, _, _, err = cmd.ExecuteCommandC(New(), "refs/heads/main")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("missing arguments", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "accepts 1 arg(s), received 0")
	})

	t.Run("no signing key configured", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		err = r.SetGitConfig("gpg.format", "ssh")
		require.NoError(t, err)

		err = r.SetGitConfig("user.signingkey", "")
		require.NoError(t, err)

		libRepo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		treeBuilder := gitinterface.NewTreeBuilder(r)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)

		_, err = r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.NoError(t, err)

		err = libRepo.RecordRSLEntryForReference(t.Context(), "refs/heads/main", false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		err = r.SetReference("refs/heads/main", gitinterface.ZeroHash)
		require.NoError(t, err)

		_, err = r.Commit(emptyTreeHash, "refs/heads/main", "Real initial commit\n", false)
		require.NoError(t, err)

		err = libRepo.RecordRSLEntryForReference(t.Context(), "refs/heads/main", false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		_, _, _, err = cmd.ExecuteCommandC(New(), "refs/heads/main")
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)
	})
}

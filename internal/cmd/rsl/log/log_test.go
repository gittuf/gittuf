// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDirWritable(t *testing.T, dir string) {
	t.Helper()
	t.Cleanup(func() {
		_ = filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
			if err == nil {
				_ = os.Chmod(path, 0o700) //nolint:gosec
			}
			return nil
		})
	})
}

func TestLog(t *testing.T) {
	t.Run("invalid repository path", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "not a git repository")
	})

	t.Run("repository without RSL initialized", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		_ = gitinterface.CreateTestGitRepository(t, tmpDir, false)

		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to find RSL entry")
	})

	t.Run("success without filters", func(t *testing.T) {
		tmpDir := t.TempDir()
		makeDirWritable(t, tmpDir)
		r := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		t.Chdir(tmpDir)

		libRepo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		treeBuilder := gitinterface.NewTreeBuilder(r)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)

		_, err = r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.NoError(t, err)

		err = libRepo.RecordRSLEntryForReference(t.Context(), "refs/heads/main", false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		_, out, _, err := cmd.ExecuteCommandC(New())
		assert.NoError(t, err)
		assert.Contains(t, out.String(), "refs/heads/main")
	})

	t.Run("filter by existing ref", func(t *testing.T) {
		tmpDir := t.TempDir()
		makeDirWritable(t, tmpDir)
		r := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		t.Chdir(tmpDir)

		libRepo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		treeBuilder := gitinterface.NewTreeBuilder(r)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)

		_, err = r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.NoError(t, err)

		err = libRepo.RecordRSLEntryForReference(t.Context(), "refs/heads/main", false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		_, out, _, err := cmd.ExecuteCommandC(New(), "--ref", "main")
		assert.NoError(t, err)
		assert.Contains(t, out.String(), "refs/heads/main")
	})

	t.Run("filter by non-existent ref", func(t *testing.T) {
		tmpDir := t.TempDir()
		makeDirWritable(t, tmpDir)
		r := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		t.Chdir(tmpDir)

		libRepo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		treeBuilder := gitinterface.NewTreeBuilder(r)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)

		_, err = r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.NoError(t, err)

		err = libRepo.RecordRSLEntryForReference(t.Context(), "refs/heads/main", false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		_, _, _, err = cmd.ExecuteCommandC(New(), "--ref", "nonexistent")
		assert.ErrorContains(t, err, "requested Git reference not found")
	})
}

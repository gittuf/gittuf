// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package annotate

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnotate(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "some-entry-id", "-m", "annotation message", "--local-only")
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "-m", "annotation message", "--local-only")
		assert.ErrorContains(t, err, "requires at least 1 arg")
	})

	t.Run("missing message flag", func(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "some-entry-id", "--local-only")
		assert.ErrorContains(t, err, "required flag(s) \"message\" not set")
	})

	t.Run("missing remote-name or local-only", func(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "some-entry-id", "-m", "annotation message")
		assert.ErrorContains(t, err, "at least one of the flags in the group [remote-name local-only] is required")
	})

	t.Run("both remote-name and local-only", func(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "some-entry-id", "-m", "annotation message", "--local-only", "--remote-name", "origin")
		assert.ErrorContains(t, err, "if any flags in the group [remote-name local-only] are set")
	})

	t.Run("unknown entry ID", func(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), gitinterface.ZeroHash.String(), "-m", "annotation message", "--local-only")
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
	})

	t.Run("successful local annotation", func(t *testing.T) {
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

		libRepo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		treeBuilder := gitinterface.NewTreeBuilder(r)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)

		_, err = r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.NoError(t, err)

		err = libRepo.RecordRSLEntryForReference(t.Context(), "refs/heads/main", false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		latestEntry, err := rsl.GetLatestEntry(r)
		require.NoError(t, err)
		entryID := latestEntry.GetID()

		_, _, _, err = cmd.ExecuteCommandC(New(), entryID.String(), "-m", "test annotation message", "--local-only")
		assert.NoError(t, err)

		latestEntry, err = rsl.GetLatestEntry(r)
		require.NoError(t, err)
		assert.IsType(t, &rsl.AnnotationEntry{}, latestEntry)

		annotation := latestEntry.(*rsl.AnnotationEntry)
		assert.Equal(t, "test annotation message", annotation.Message)
		assert.Equal(t, []gitinterface.Hash{entryID}, annotation.RSLEntryIDs)
		assert.False(t, annotation.Skip)
	})

	t.Run("successful local annotation with skip", func(t *testing.T) {
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

		libRepo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		treeBuilder := gitinterface.NewTreeBuilder(r)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)

		_, err = r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.NoError(t, err)

		err = libRepo.RecordRSLEntryForReference(t.Context(), "refs/heads/main", false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		latestEntry, err := rsl.GetLatestEntry(r)
		require.NoError(t, err)
		entryID := latestEntry.GetID()

		_, _, _, err = cmd.ExecuteCommandC(New(), entryID.String(), "-m", "test skip message", "--skip", "--local-only")
		assert.NoError(t, err)

		latestEntry, err = rsl.GetLatestEntry(r)
		require.NoError(t, err)
		assert.IsType(t, &rsl.AnnotationEntry{}, latestEntry)

		annotation := latestEntry.(*rsl.AnnotationEntry)
		assert.Equal(t, "test skip message", annotation.Message)
		assert.Equal(t, []gitinterface.Hash{entryID}, annotation.RSLEntryIDs)
		assert.True(t, annotation.Skip)
	})
}

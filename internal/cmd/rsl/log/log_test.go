// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"bytes"
	"io"
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

func TestLog(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("no RSL entries", func(t *testing.T) {
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
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
	})

	t.Run("success", func(t *testing.T) {
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

		commitID, err := r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.NoError(t, err)

		err = libRepo.RecordRSLEntryForReference(t.Context(), "refs/heads/main", false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		oldStdout := os.Stdout
		pipeReader, pipeWriter, err := os.Pipe()
		require.NoError(t, err)
		os.Stdout = pipeWriter

		_, _, _, cmdErr := cmd.ExecuteCommandC(New())

		pipeWriter.Close()
		os.Stdout = oldStdout

		require.NoError(t, cmdErr)

		var buf bytes.Buffer
		_, err = io.Copy(&buf, pipeReader)
		require.NoError(t, err)

		stdout := buf.String()
		assert.Contains(t, stdout, "refs/heads/main")
		assert.Contains(t, stdout, commitID.String())
	})

	t.Run("success with ref filter", func(t *testing.T) {
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

		commitIDMain, err := r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.NoError(t, err)

		err = libRepo.RecordRSLEntryForReference(t.Context(), "refs/heads/main", false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		commitIDFeature, err := r.Commit(emptyTreeHash, "refs/heads/feature", "Feature commit\n", false)
		require.NoError(t, err)

		err = libRepo.RecordRSLEntryForReference(t.Context(), "refs/heads/feature", false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		oldStdout := os.Stdout
		pipeReader, pipeWriter, err := os.Pipe()
		require.NoError(t, err)
		os.Stdout = pipeWriter

		// Filter for main only
		_, _, _, cmdErr := cmd.ExecuteCommandC(New(), "--ref", "refs/heads/main")

		pipeWriter.Close()
		os.Stdout = oldStdout

		require.NoError(t, cmdErr)

		var buf bytes.Buffer
		_, err = io.Copy(&buf, pipeReader)
		require.NoError(t, err)

		stdout := buf.String()
		assert.Contains(t, stdout, "refs/heads/main")
		assert.Contains(t, stdout, commitIDMain.String())
		assert.NotContains(t, stdout, "refs/heads/feature")
		assert.NotContains(t, stdout, commitIDFeature.String())
	})
}

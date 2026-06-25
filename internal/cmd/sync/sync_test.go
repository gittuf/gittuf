// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestSync(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(currentDir)
		}()

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("no remote", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(currentDir)
		}()

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		_, _, _, err = cmd.ExecuteCommandC(New(), "custom-remote")
		assert.ErrorContains(t, err, "No such remote")
	})

	t.Run("diverged refs and overwrite", func(t *testing.T) {
		refName := "refs/heads/main"

		// 1. Setup Remote Repo
		remoteTmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, remoteTmpDir, false)

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := remoteR.Commit(emptyTreeHash, refName, "Remote commit", false); err != nil {
			t.Fatal(err)
		}

		// We need a dummy gittuf repo to record RSL
		remoteRepo, err := gittuf.LoadRepository(remoteTmpDir)
		if err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(t.Context(), refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// 2. Setup Local Repo (Clone remote)
		localTmpDir := filepath.Join(t.TempDir(), "local-sync-test")
		localR, err := gitinterface.CloneAndFetchRepository(remoteTmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := localR.SetGitConfig("user.name", "Jane Doe"); err != nil {
			t.Fatal(err)
		}
		if err := localR.SetGitConfig("user.email", "jane.doe@example.com"); err != nil {
			t.Fatal(err)
		}

		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(currentDir)
		}()

		// 3. Make Remote and Local Diverge
		// Remote Action:
		if _, err := remoteRepo.GetGitRepository().Commit(emptyTreeHash, refName, "Another remote commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(t.Context(), refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Local Action:
		localRepo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := localRepo.GetGitRepository().Commit(emptyTreeHash, refName, "Local commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(t.Context(), refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// 4. Test Sync without --overwrite (should catch divergence)
		_, stdOut, _, err := cmd.ExecuteCommandC(New())
		assert.NoError(t, err)

		outputStr := stdOut.String()
		assert.Contains(t, outputStr, "References have diverged:")
		assert.Contains(t, outputStr, "To apply upstream changes locally, rerun the command with --overwrite")

		// 5. Test Sync with --overwrite (should successfully overwrite)
		_, _, _, err = cmd.ExecuteCommandC(New(), "--overwrite")
		assert.NoError(t, err)
	})
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordRSLEntryForReference(t *testing.T) {
	tempDir := t.TempDir()
	r := gitinterface.CreateTestGitRepository(t, tempDir, false)

	repo := &Repository{r: r}

	treeBuilder := gitinterface.NewTreeBuilder(repo.r)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}
	commitID, err := repo.r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.RecordRSLEntryForReference(testCtx, "refs/heads/main", false, rslopts.WithRecordLocalOnly()); err != nil {
		t.Fatal(err)
	}

	entryT, err := rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}

	entry, ok := entryT.(*rsl.ReferenceEntry)
	if !ok {
		t.Fatal(fmt.Errorf("invalid entry type"))
	}
	assert.Equal(t, "refs/heads/main", entry.RefName)
	assert.Equal(t, commitID, entry.TargetID)

	newCommitID, err := repo.r.Commit(emptyTreeHash, "refs/heads/main", "Another commit\n", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.RecordRSLEntryForReference(testCtx, "main", false, rslopts.WithRecordLocalOnly()); err != nil {
		t.Fatal(err)
	}

	rslRef, err := repo.r.GetReference(rsl.Ref)
	if err != nil {
		t.Fatal(err)
	}

	entryT, err = rsl.GetEntry(repo.r, rslRef)
	if err != nil {
		t.Fatal(err)
	}

	entry, ok = entryT.(*rsl.ReferenceEntry)
	if !ok {
		t.Fatal(fmt.Errorf("invalid entry type"))
	}
	assert.Equal(t, "refs/heads/main", entry.RefName)
	assert.Equal(t, newCommitID, entry.TargetID)

	err = repo.RecordRSLEntryForReference(testCtx, "main", false, rslopts.WithRecordLocalOnly())
	assert.Nil(t, err)

	entryT, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	// check that a duplicate entry has not been created
	assert.Equal(t, entry.GetID(), entryT.GetID())

	// Record entry for a different dst ref
	err = repo.RecordRSLEntryForReference(testCtx, "refs/heads/main", false, rslopts.WithOverrideRefName("refs/heads/not-main"), rslopts.WithRecordLocalOnly())
	assert.Nil(t, err)

	entryT, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok = entryT.(*rsl.ReferenceEntry)
	if !ok {
		t.Fatal(fmt.Errorf("invalid entry type"))
	}

	assert.Equal(t, newCommitID, entry.TargetID)
	assert.Equal(t, "refs/heads/not-main", entry.RefName)

	// Record entry for a different dst ref and skip check for duplicate
	currentEntryID := entry.GetID()
	err = repo.RecordRSLEntryForReference(testCtx, "refs/heads/main", false, rslopts.WithOverrideRefName("refs/heads/not-main"), rslopts.WithSkipCheckForDuplicateEntry(), rslopts.WithRecordLocalOnly())
	assert.Nil(t, err)

	entryT, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok = entryT.(*rsl.ReferenceEntry)
	if !ok {
		t.Fatal(fmt.Errorf("invalid entry type"))
	}

	assert.NotEqual(t, currentEntryID, entry.GetID())
	assert.Equal(t, newCommitID, entry.TargetID)
	assert.Equal(t, "refs/heads/not-main", entry.RefName)
}

func TestRecordRSLEntryForReferenceAtTarget(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	tests := map[string]struct {
		keyBytes []byte
	}{
		"using GPG key":       {keyBytes: gpgKeyBytes},
		"using RSA SSH key":   {keyBytes: rsaKeyBytes},
		"using ECDSA ssh key": {keyBytes: ecdsaKeyBytes},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			r := gitinterface.CreateTestGitRepository(t, tmpDir, false)
			repo := &Repository{r: r}

			treeBuilder := gitinterface.NewTreeBuilder(repo.r)
			emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
			if err != nil {
				t.Fatal(err)
			}
			commitID, err := repo.r.Commit(emptyTreeHash, refName, "Test commit", false)
			if err != nil {
				t.Fatal(err)
			}

			err = repo.RecordRSLEntryForReferenceAtTarget(refName, commitID.String(), test.keyBytes)
			assert.Nil(t, err)

			latestEntry, err := rsl.GetLatestEntry(repo.r)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, refName, latestEntry.(*rsl.ReferenceEntry).RefName)
			assert.Equal(t, commitID, latestEntry.(*rsl.ReferenceEntry).TargetID)

			// Now checkout another branch, add another commit
			if err := repo.r.SetReference(anotherRefName, commitID); err != nil {
				t.Fatal(err)
			}
			newCommitID, err := repo.r.Commit(emptyTreeHash, anotherRefName, "Commit on feature branch", false)
			if err != nil {
				t.Fatal(err)
			}

			// We record an RSL entry for the commit in the new branch
			err = repo.RecordRSLEntryForReferenceAtTarget(anotherRefName, newCommitID.String(), test.keyBytes)
			assert.Nil(t, err)

			// Let's record a couple more commits and use the older of the two
			commitID, err = repo.r.Commit(emptyTreeHash, refName, "Another commit", false)
			if err != nil {
				t.Fatal(err)
			}
			_, err = repo.r.Commit(emptyTreeHash, refName, "Latest commit", false)
			if err != nil {
				t.Fatal(err)
			}

			err = repo.RecordRSLEntryForReferenceAtTarget(refName, commitID.String(), test.keyBytes)
			assert.Nil(t, err)

			// Let's record a couple more commits and add an entry with a
			// different dstRefName to the first rather than latest commit
			commitID, err = repo.r.Commit(emptyTreeHash, refName, "Another commit", false)
			if err != nil {
				t.Fatal(err)
			}
			_, err = repo.r.Commit(emptyTreeHash, refName, "Latest commit", false)
			if err != nil {
				t.Fatal(err)
			}

			err = repo.RecordRSLEntryForReferenceAtTarget(refName, commitID.String(), test.keyBytes, rslopts.WithOverrideRefName(anotherRefName))
			assert.Nil(t, err)

			latestEntry, err = rsl.GetLatestEntry(repo.r)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, anotherRefName, latestEntry.(*rsl.ReferenceEntry).RefName)
			assert.Equal(t, commitID, latestEntry.(*rsl.ReferenceEntry).TargetID)
		})
	}
}

func TestRecordRSLAnnotation(t *testing.T) {
	tempDir := t.TempDir()
	r := gitinterface.CreateTestGitRepository(t, tempDir, false)

	repo := &Repository{r: r}

	err := repo.RecordRSLAnnotation(testCtx, []string{gitinterface.ZeroHash.String()}, false, "test annotation", false, rslopts.WithAnnotateLocalOnly())
	assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)

	treeBuilder := gitinterface.NewTreeBuilder(repo.r)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordRSLEntryForReference(testCtx, "refs/heads/main", false, rslopts.WithRecordLocalOnly()); err != nil {
		t.Fatal(err)
	}

	latestEntry, err := rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	entryID := latestEntry.GetID()

	err = repo.RecordRSLAnnotation(testCtx, []string{entryID.String()}, false, "test annotation", false, rslopts.WithAnnotateLocalOnly())
	assert.Nil(t, err)

	latestEntry, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	assert.IsType(t, &rsl.AnnotationEntry{}, latestEntry)

	annotation := latestEntry.(*rsl.AnnotationEntry)
	assert.Equal(t, "test annotation", annotation.Message)
	assert.Equal(t, []gitinterface.Hash{entryID}, annotation.RSLEntryIDs)
	assert.False(t, annotation.Skip)

	err = repo.RecordRSLAnnotation(testCtx, []string{entryID.String()}, true, "skip annotation", false, rslopts.WithAnnotateLocalOnly())
	assert.Nil(t, err)

	latestEntry, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	assert.IsType(t, &rsl.AnnotationEntry{}, latestEntry)

	annotation = latestEntry.(*rsl.AnnotationEntry)
	assert.Equal(t, "skip annotation", annotation.Message)
	assert.Equal(t, []gitinterface.Hash{entryID}, annotation.RSLEntryIDs)
	assert.True(t, annotation.Skip)
}

func TestReconcileLocalRSLWithRemote(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	t.Run("remote has updates for local", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)

		// Simulate more remote actions
		if _, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		originalRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.ReconcileLocalRSLWithRemote(testCtx, remoteName, false)
		assert.Nil(t, err)

		currentRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		// Local RSL must now be updated to match remote
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)
		assert.NotEqual(t, originalRSLTip, currentRSLTip)
	})

	t.Run("remote has no updates for local", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

		originalRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.ReconcileLocalRSLWithRemote(testCtx, remoteName, false)
		assert.Nil(t, err)

		currentRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, originalRSLTip, currentRSLTip)
	})

	t.Run("local is ahead of remote", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		// Simulate local actions
		if _, err := localR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		originalLocalRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}
		originalRemoteRSLTip, err := remoteRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.ReconcileLocalRSLWithRemote(testCtx, remoteName, false)
		assert.Nil(t, err)

		currentLocalRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}
		currentRemoteRSLTip, err := remoteRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		// No change to local AND no change to remote
		assert.Equal(t, originalLocalRSLTip, currentLocalRSLTip)
		assert.Equal(t, originalRemoteRSLTip, currentRemoteRSLTip)
	})

	t.Run("remote and local have diverged", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		// Simulate remote actions
		if _, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions
		if _, err := localRepo.r.Commit(emptyTreeHash, anotherRefName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(testCtx, anotherRefName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		originalLocalRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}
		originalRemoteRSLTip, err := remoteRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.ReconcileLocalRSLWithRemote(testCtx, remoteName, false)
		assert.Nil(t, err)

		currentLocalRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}
		currentRemoteRSLTip, err := remoteRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		// Remote must not have changed
		assert.Equal(t, originalRemoteRSLTip, currentRemoteRSLTip)

		// The current remote tip must be the parent of the current
		// local tip
		parents, err := localRepo.r.GetCommitParentIDs(currentLocalRSLTip)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, currentRemoteRSLTip, parents[0])

		// The current local tip and original local tip must have same
		// entry ref and target ID
		originalEntry, err := rsl.GetEntry(localRepo.r, originalLocalRSLTip)
		if err != nil {
			t.Fatal(err)
		}
		currentEntry, err := rsl.GetEntry(localRepo.r, currentLocalRSLTip)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, originalEntry.(*rsl.ReferenceEntry).RefName, currentEntry.(*rsl.ReferenceEntry).RefName)
		assert.Equal(t, originalEntry.(*rsl.ReferenceEntry).TargetID, currentEntry.(*rsl.ReferenceEntry).TargetID)
	})

	t.Run("remote and local have diverged but modify same ref", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		// Simulate remote actions
		if _, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions -- NOT anotherRefname here
		if _, err := localRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		originalLocalRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}
		originalRemoteRSLTip, err := remoteRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.ReconcileLocalRSLWithRemote(testCtx, remoteName, false)
		assert.ErrorContains(t, err, "changes to the same ref")

		currentLocalRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}
		currentRemoteRSLTip, err := remoteRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		// Neither RSL should have changed
		assert.Equal(t, originalRemoteRSLTip, currentRemoteRSLTip)
		assert.Equal(t, originalLocalRSLTip, currentLocalRSLTip)
	})
}

func TestSync(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"

	t.Run("local and remote are identical", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, true)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)

		divergedRefs, err := localRepo.Sync(testCtx, remoteName, false, false)
		assert.Nil(t, err)
		assert.Empty(t, divergedRefs)
	})

	t.Run("local is strictly ahead of remote", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, true)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)

		// Simulate local actions
		if _, err := localRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		originalRSLTip, err := remoteRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		divergedRefs, err := localRepo.Sync(testCtx, remoteName, false, false)
		assert.Nil(t, err)
		assert.Empty(t, divergedRefs)

		currentRSLTip, err := remoteRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		// Remote RSL must now be updated to match local
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, refName)
		assert.NotEqual(t, originalRSLTip, currentRSLTip)
	})

	t.Run("local is strictly behind remote", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)

		// Simulate more remote actions
		if _, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		originalRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		divergedRefs, err := localRepo.Sync(testCtx, remoteName, false, false)
		assert.Nil(t, err)
		assert.Empty(t, divergedRefs)

		currentRSLTip, err := localRepo.r.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}

		// Local RSL must now be updated to match remote
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, refName)
		assert.NotEqual(t, originalRSLTip, currentRSLTip)
	})

	t.Run("local RSL has diverged, not allowed to overwrite", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)

		// Simulate more remote actions
		if _, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions
		if _, err := localRepo.r.Commit(emptyTreeHash, refName, "Local test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		divergedRefs, err := localRepo.Sync(testCtx, remoteName, false, false)
		assert.ErrorIs(t, err, ErrDivergedRefs)
		assert.Contains(t, divergedRefs, rsl.Ref)
	})

	t.Run("local ref (not RSL) has diverged, not allowed to overwrite", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)

		// Simulate more remote actions
		if _, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions
		if _, err := localRepo.r.Commit(emptyTreeHash, refName, "Local test commit", false); err != nil {
			t.Fatal(err)
		}

		divergedRefs, err := localRepo.Sync(testCtx, remoteName, false, false)
		assert.ErrorIs(t, err, ErrDivergedRefs)
		assert.Contains(t, divergedRefs, refName)
	})

	t.Run("local RSL has diverged, allowed to overwrite", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)

		// Simulate more remote actions
		if _, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions
		if _, err := localRepo.r.Commit(emptyTreeHash, refName, "Local test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		divergedRefs, err := localRepo.Sync(testCtx, remoteName, true, false)
		assert.Nil(t, err)
		assert.Empty(t, divergedRefs)

		// Local RSL must now be updated to match remote
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, refName)
	})

	t.Run("local ref (not RSL) has diverged, allowed to overwrite", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)

		// Simulate more remote actions
		if _, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions
		if _, err := localRepo.r.Commit(emptyTreeHash, refName, "Local test commit", false); err != nil {
			t.Fatal(err)
		}

		divergedRefs, err := localRepo.Sync(testCtx, remoteName, true, false)
		assert.Nil(t, err)
		assert.Empty(t, divergedRefs)

		// Local RSL must now be updated to match remote
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, refName)
	})
}

func TestPushRSL(t *testing.T) {
	remoteName := "origin"

	t.Run("successful push", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		remoteRepoR := gitinterface.CreateTestGitRepository(t, remoteTmpDir, false)

		localRepo := createTestRepositoryWithPolicy(t, "")
		if err := localRepo.r.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.PushRSL(remoteName)
		assert.Nil(t, err)

		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepoR, rsl.Ref)

		// No updates, successful push
		err = localRepo.PushRSL(remoteName)
		assert.Nil(t, err)
	})

	t.Run("divergent RSLs, unsuccessful push", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		remoteRepoR := gitinterface.CreateTestGitRepository(t, remoteTmpDir, false)

		if err := rsl.NewReferenceEntry(policy.PolicyRef, gitinterface.ZeroHash).Commit(remoteRepoR, false); err != nil {
			t.Fatal(err)
		}

		localRepo := createTestRepositoryWithPolicy(t, "")
		if err := localRepo.r.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.PushRSL(remoteName)
		assert.ErrorIs(t, err, ErrPushingRSL)
	})
}

func TestPullRSL(t *testing.T) {
	remoteName := "origin"

	t.Run("successful pull", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		remoteRepo := createTestRepositoryWithPolicy(t, remoteTmpDir)

		localTmpDir := t.TempDir()
		localRepoR := gitinterface.CreateTestGitRepository(t, localTmpDir, false)
		localRepo := &Repository{r: localRepoR}
		if err := localRepo.r.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.PullRSL(remoteName)
		assert.Nil(t, err)

		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo.r, rsl.Ref)

		// No updates, successful pull
		err = localRepo.PullRSL(remoteName)
		assert.Nil(t, err)
	})

	t.Run("divergent RSLs, unsuccessful pull", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		createTestRepositoryWithPolicy(t, remoteTmpDir)

		localTmpDir := t.TempDir()
		localRepoR := gitinterface.CreateTestGitRepository(t, localTmpDir, false)
		localRepo := &Repository{r: localRepoR}
		if err := localRepo.r.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewReferenceEntry(policy.PolicyRef, gitinterface.ZeroHash).Commit(localRepo.r, false); err != nil {
			t.Fatal(err)
		}

		err := localRepo.PullRSL(remoteName)
		assert.ErrorIs(t, err, ErrPullingRSL)
	})
}

func TestPropagateChangesFromUpstreamRepositories(t *testing.T) {
	t.Run("single upstream repo", func(t *testing.T) {
		// Create upstreamRepo
		upstreamRepoLocation := t.TempDir()
		upstreamRepo := createTestRepositoryWithRoot(t, upstreamRepoLocation)

		downstreamRepoLocation := t.TempDir()
		downstreamRepo := createTestRepositoryWithRoot(t, downstreamRepoLocation)

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		refName := "refs/heads/main"
		localPath := "upstream"
		if err := downstreamRepo.AddPropagationDirective(testCtx, signer, "test", upstreamRepoLocation, refName, "", refName, localPath, false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.StagePolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.ApplyPolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}

		err := downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.NotNil(t, err) // TODO: upstream doesn't have main at all

		// Add things to upstreamRepo
		blobAID, err := upstreamRepo.r.WriteBlob([]byte("a"))
		if err != nil {
			t.Fatal(err)
		}

		blobBID, err := upstreamRepo.r.WriteBlob([]byte("b"))
		if err != nil {
			t.Fatal(err)
		}

		upstreamTreeBuilder := gitinterface.NewTreeBuilder(upstreamRepo.r)
		upstreamRootTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := upstreamRepo.r.Commit(upstreamRootTreeID, refName, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := upstreamRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}
		upstreamEntry, err := rsl.GetLatestEntry(upstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}

		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		// TODO: should propagation result in a new local ref?
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Add things to downstreamRepo
		blobAID, err = downstreamRepo.r.WriteBlob([]byte("a"))
		if err != nil {
			t.Fatal(err)
		}

		blobBID, err = downstreamRepo.r.WriteBlob([]byte("b"))
		if err != nil {
			t.Fatal(err)
		}

		downstreamTreeBuilder := gitinterface.NewTreeBuilder(downstreamRepo.r)
		downstreamRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("foo/b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := downstreamRepo.r.Commit(downstreamRootTreeID, refName, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.Nil(t, err)

		latestEntry, err := rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}
		propagationEntry, isPropagationEntry := latestEntry.(*rsl.PropagationEntry)
		if !isPropagationEntry {
			t.Fatal("unexpected entry type in downstream repo")
		}
		assert.Equal(t, upstreamRepoLocation, propagationEntry.UpstreamRepository)
		assert.Equal(t, upstreamEntry.GetID(), propagationEntry.UpstreamEntryID)

		downstreamRootTreeID, err = downstreamRepo.r.GetCommitTreeID(propagationEntry.TargetID)
		if err != nil {
			t.Fatal(err)
		}
		pathTreeID, err := downstreamRepo.r.GetPathIDInTree(localPath, downstreamRootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		// Check the subtree ID in downstream repo matches upstream root tree ID
		assert.Equal(t, upstreamRootTreeID, pathTreeID)

		// Check the downstream tree still contains other items
		expectedRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("foo/b", blobBID),
			gitinterface.NewEntryBlob("upstream/a", blobAID),
			gitinterface.NewEntryBlob("upstream/b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectedRootTreeID, downstreamRootTreeID)

		// Nothing to propagate, check that a new entry has not been added in the downstreamRepo
		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.Nil(t, err)

		latestEntry, err = rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, propagationEntry.GetID(), latestEntry.GetID())
	})

	t.Run("single upstream repo, multiple upstream refs into same downstream ref", func(t *testing.T) {
		// Create upstreamRepo
		upstreamRepoLocation := t.TempDir()
		upstreamRepo := createTestRepositoryWithRoot(t, upstreamRepoLocation)

		downstreamRepoLocation := t.TempDir()
		downstreamRepo := createTestRepositoryWithRoot(t, downstreamRepoLocation)

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		refName1 := "refs/heads/main"
		refName2 := "refs/heads/feature"
		localPath1 := "main"
		localPath2 := "feature"
		if err := downstreamRepo.AddPropagationDirective(testCtx, signer, "test", upstreamRepoLocation, refName1, "", refName1, localPath1, false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.AddPropagationDirective(testCtx, signer, "test", upstreamRepoLocation, refName2, "", refName1, localPath2, false); err != nil {
			t.Fatal(err)
		}

		if err := downstreamRepo.StagePolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.ApplyPolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}

		err := downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.NotNil(t, err) // TODO: upstream doesn't have main at all

		// Add things to upstreamRepo
		blobAID, err := upstreamRepo.r.WriteBlob([]byte("a"))
		if err != nil {
			t.Fatal(err)
		}

		blobBID, err := upstreamRepo.r.WriteBlob([]byte("b"))
		if err != nil {
			t.Fatal(err)
		}

		upstreamTreeBuilder := gitinterface.NewTreeBuilder(upstreamRepo.r)
		upstreamRootTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := upstreamRepo.r.Commit(upstreamRootTreeID, refName1, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := upstreamRepo.RecordRSLEntryForReference(testCtx, refName1, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}
		if _, err := upstreamRepo.r.Commit(upstreamRootTreeID, refName2, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := upstreamRepo.RecordRSLEntryForReference(testCtx, refName2, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}
		upstreamEntry2, err := rsl.GetLatestEntry(upstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}
		upstreamEntry1, err := rsl.GetParentForEntry(upstreamRepo.r, upstreamEntry2)
		if err != nil {
			t.Fatal(err)
		}

		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		// TODO: should propagation result in a new local ref?
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Add things to downstreamRepo
		blobAID, err = downstreamRepo.r.WriteBlob([]byte("a"))
		if err != nil {
			t.Fatal(err)
		}

		blobBID, err = downstreamRepo.r.WriteBlob([]byte("b"))
		if err != nil {
			t.Fatal(err)
		}

		downstreamTreeBuilder := gitinterface.NewTreeBuilder(downstreamRepo.r)
		downstreamRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("foo/b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := downstreamRepo.r.Commit(downstreamRootTreeID, refName1, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.RecordRSLEntryForReference(testCtx, refName1, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.Nil(t, err)

		latestEntry, err := rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}
		priorEntry, err := rsl.GetParentForEntry(downstreamRepo.r, latestEntry)
		if err != nil {
			t.Fatal(err)
		}
		propagationEntry2, isPropagationEntry := latestEntry.(*rsl.PropagationEntry)
		if !isPropagationEntry {
			t.Fatal("unexpected entry type in downstream repo")
		}
		propagationEntry1, isPropagationEntry := priorEntry.(*rsl.PropagationEntry)
		if !isPropagationEntry {
			t.Fatal("unexpected entry type in downstream repo")
		}
		assert.Equal(t, upstreamRepoLocation, propagationEntry1.UpstreamRepository)
		assert.Equal(t, upstreamRepoLocation, propagationEntry2.UpstreamRepository)
		assert.Equal(t, upstreamEntry1.GetID(), propagationEntry1.UpstreamEntryID)
		assert.Equal(t, upstreamEntry2.GetID(), propagationEntry2.UpstreamEntryID)

		downstreamRootTreeID, err = downstreamRepo.r.GetCommitTreeID(propagationEntry2.TargetID)
		if err != nil {
			t.Fatal(err)
		}
		pathTree1ID, err := downstreamRepo.r.GetPathIDInTree(localPath1, downstreamRootTreeID)
		if err != nil {
			t.Fatal(err)
		}
		pathTree2ID, err := downstreamRepo.r.GetPathIDInTree(localPath2, downstreamRootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		// Check the subtree IDs in downstream repo matches upstream root tree IDs
		assert.Equal(t, upstreamRootTreeID, pathTree1ID)
		assert.Equal(t, upstreamRootTreeID, pathTree2ID)

		// Check the downstream tree still contains other items
		expectedRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("foo/b", blobBID),
			gitinterface.NewEntryBlob("main/a", blobAID),
			gitinterface.NewEntryBlob("main/b", blobBID),
			gitinterface.NewEntryBlob("feature/a", blobAID),
			gitinterface.NewEntryBlob("feature/b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectedRootTreeID, downstreamRootTreeID)

		// Nothing to propagate, check that a new entry has not been added in the downstreamRepo
		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.Nil(t, err)

		latestEntry, err = rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, propagationEntry2.GetID(), latestEntry.GetID())
	})

	t.Run("single upstream repo, multiple upstream refs into different downstream refs", func(t *testing.T) {
		// Create upstreamRepo
		upstreamRepoLocation := t.TempDir()
		upstreamRepo := createTestRepositoryWithRoot(t, upstreamRepoLocation)

		downstreamRepoLocation := t.TempDir()
		downstreamRepo := createTestRepositoryWithRoot(t, downstreamRepoLocation)

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		refName1 := "refs/heads/main"
		refName2 := "refs/heads/feature"
		localPath := "upstream"
		if err := downstreamRepo.AddPropagationDirective(testCtx, signer, "test", upstreamRepoLocation, refName1, "", refName1, localPath, false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.AddPropagationDirective(testCtx, signer, "test", upstreamRepoLocation, refName2, "", refName2, localPath, false); err != nil {
			t.Fatal(err)
		}

		if err := downstreamRepo.StagePolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.ApplyPolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}

		err := downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.NotNil(t, err) // TODO: upstream doesn't have main at all

		// Add things to upstreamRepo
		blobAID, err := upstreamRepo.r.WriteBlob([]byte("a"))
		if err != nil {
			t.Fatal(err)
		}

		blobBID, err := upstreamRepo.r.WriteBlob([]byte("b"))
		if err != nil {
			t.Fatal(err)
		}

		upstreamTreeBuilder := gitinterface.NewTreeBuilder(upstreamRepo.r)
		upstreamRootTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := upstreamRepo.r.Commit(upstreamRootTreeID, refName1, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := upstreamRepo.RecordRSLEntryForReference(testCtx, refName1, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}
		if _, err := upstreamRepo.r.Commit(upstreamRootTreeID, refName2, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := upstreamRepo.RecordRSLEntryForReference(testCtx, refName2, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}
		upstreamEntry2, err := rsl.GetLatestEntry(upstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}
		upstreamEntry1, err := rsl.GetParentForEntry(upstreamRepo.r, upstreamEntry2)
		if err != nil {
			t.Fatal(err)
		}

		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		// TODO: should propagation result in a new local ref?
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Add things to downstreamRepo
		blobAID, err = downstreamRepo.r.WriteBlob([]byte("a"))
		if err != nil {
			t.Fatal(err)
		}

		blobBID, err = downstreamRepo.r.WriteBlob([]byte("b"))
		if err != nil {
			t.Fatal(err)
		}

		downstreamTreeBuilder := gitinterface.NewTreeBuilder(downstreamRepo.r)
		downstreamRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("foo/b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := downstreamRepo.r.Commit(downstreamRootTreeID, refName1, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.RecordRSLEntryForReference(testCtx, refName1, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}
		if _, err := downstreamRepo.r.Commit(downstreamRootTreeID, refName2, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.RecordRSLEntryForReference(testCtx, refName2, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.Nil(t, err)

		latestEntry, err := rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}
		priorEntry, err := rsl.GetParentForEntry(downstreamRepo.r, latestEntry)
		if err != nil {
			t.Fatal(err)
		}
		propagationEntry2, isPropagationEntry := latestEntry.(*rsl.PropagationEntry)
		if !isPropagationEntry {
			t.Fatal("unexpected entry type in downstream repo")
		}
		propagationEntry1, isPropagationEntry := priorEntry.(*rsl.PropagationEntry)
		if !isPropagationEntry {
			t.Fatal("unexpected entry type in downstream repo")
		}
		assert.Equal(t, upstreamRepoLocation, propagationEntry1.UpstreamRepository)
		assert.Equal(t, upstreamRepoLocation, propagationEntry2.UpstreamRepository)
		assert.Equal(t, upstreamEntry1.GetID(), propagationEntry1.UpstreamEntryID)
		assert.Equal(t, upstreamEntry2.GetID(), propagationEntry2.UpstreamEntryID)
		assert.Equal(t, refName1, propagationEntry1.RefName)
		assert.Equal(t, refName2, propagationEntry2.RefName)

		// Check the downstream tree still contains other items
		expectedRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("foo/b", blobBID),
			gitinterface.NewEntryBlob("upstream/a", blobAID),
			gitinterface.NewEntryBlob("upstream/b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}

		downstreamRootTreeID, err = downstreamRepo.r.GetCommitTreeID(propagationEntry2.TargetID)
		if err != nil {
			t.Fatal(err)
		}
		pathTreeID, err := downstreamRepo.r.GetPathIDInTree(localPath, downstreamRootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		// Check the subtree ID in downstream repo matches upstream root tree ID
		assert.Equal(t, upstreamRootTreeID, pathTreeID)
		// Check the tree as a whole is as expected
		assert.Equal(t, expectedRootTreeID, downstreamRootTreeID)

		// Do the same thing for the other propagation entry's tree (this is a different ref!)
		downstreamRootTreeID, err = downstreamRepo.r.GetCommitTreeID(propagationEntry1.TargetID)
		if err != nil {
			t.Fatal(err)
		}
		pathTreeID, err = downstreamRepo.r.GetPathIDInTree(localPath, downstreamRootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		// Check the subtree ID in downstream repo matches upstream root tree ID
		assert.Equal(t, upstreamRootTreeID, pathTreeID)
		// Check the tree as a whole is as expected
		assert.Equal(t, expectedRootTreeID, downstreamRootTreeID)

		// Nothing to propagate, check that a new entry has not been added in the downstreamRepo
		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.Nil(t, err)

		latestEntry, err = rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, propagationEntry2.GetID(), latestEntry.GetID())
	})

	t.Run("multiple upstream repos", func(t *testing.T) {
		// Create upstreamRepos
		upstreamRepo1Location := t.TempDir()
		upstreamRepo1 := createTestRepositoryWithRoot(t, upstreamRepo1Location)

		upstreamRepo2Location := t.TempDir()
		upstreamRepo2 := createTestRepositoryWithRoot(t, upstreamRepo2Location)

		downstreamRepoLocation := t.TempDir()
		downstreamRepo := createTestRepositoryWithRoot(t, downstreamRepoLocation)

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		refName := "refs/heads/main"
		localPath1 := "upstream1"
		localPath2 := "upstream2"
		if err := downstreamRepo.AddPropagationDirective(testCtx, signer, "test-1", upstreamRepo1Location, refName, "", refName, localPath1, false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.AddPropagationDirective(testCtx, signer, "test-2", upstreamRepo2Location, refName, "", refName, localPath2, false); err != nil {
			t.Fatal(err)
		}

		if err := downstreamRepo.StagePolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.ApplyPolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}

		err := downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.NotNil(t, err) // TODO: upstream repos don't have main at all

		// Add things to upstreamRepos
		blobAID, err := upstreamRepo1.r.WriteBlob([]byte("a"))
		if err != nil {
			t.Fatal(err)
		}

		blobBID, err := upstreamRepo1.r.WriteBlob([]byte("b"))
		if err != nil {
			t.Fatal(err)
		}

		upstreamTreeBuilder1 := gitinterface.NewTreeBuilder(upstreamRepo1.r)
		upstreamRootTree1ID, err := upstreamTreeBuilder1.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := upstreamRepo1.r.Commit(upstreamRootTree1ID, refName, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := upstreamRepo1.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}
		upstreamEntry1, err := rsl.GetLatestEntry(upstreamRepo1.r)
		if err != nil {
			t.Fatal(err)
		}

		blobCID, err := upstreamRepo2.r.WriteBlob([]byte("c"))
		if err != nil {
			t.Fatal(err)
		}

		blobDID, err := upstreamRepo2.r.WriteBlob([]byte("d"))
		if err != nil {
			t.Fatal(err)
		}

		upstreamTreeBuilder2 := gitinterface.NewTreeBuilder(upstreamRepo2.r)
		upstreamRootTree2ID, err := upstreamTreeBuilder2.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("c", blobCID),
			gitinterface.NewEntryBlob("d", blobDID),
		})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := upstreamRepo2.r.Commit(upstreamRootTree2ID, refName, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := upstreamRepo2.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		upstreamEntry2, err := rsl.GetLatestEntry(upstreamRepo2.r)
		if err != nil {
			t.Fatal(err)
		}

		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		// TODO: should propagation result in a new local ref?
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// Add things to downstreamRepo
		blobAID, err = downstreamRepo.r.WriteBlob([]byte("a"))
		if err != nil {
			t.Fatal(err)
		}

		blobBID, err = downstreamRepo.r.WriteBlob([]byte("b"))
		if err != nil {
			t.Fatal(err)
		}

		downstreamTreeBuilder := gitinterface.NewTreeBuilder(downstreamRepo.r)
		downstreamRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("foo/b", blobBID),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := downstreamRepo.r.Commit(downstreamRootTreeID, refName, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.Nil(t, err)

		latestEntry, err := rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}
		propagationEntry2, isPropagationEntry := latestEntry.(*rsl.PropagationEntry)
		if !isPropagationEntry {
			t.Fatal("unexpected entry type in downstream repo")
		}
		priorEntry, err := rsl.GetParentForEntry(downstreamRepo.r, latestEntry)
		if err != nil {
			t.Fatal(err)
		}
		propagationEntry1, isPropagationEntry := priorEntry.(*rsl.PropagationEntry)
		if !isPropagationEntry {
			t.Fatal("unexpected entry type in downstream repo")
		}

		// Check the two propagation entries are right
		// We empty a set of items because the order of repos may change,
		// sometimes we may propagate repo A then repo B, and vice versa.
		// So instead, we empty the set of expected items based on what we see
		// in the propagation entries and ensure the set is empty so there's a
		// propagation entry for each expected item.
		expectedLocations := set.NewSetFromItems(upstreamRepo1Location, upstreamRepo2Location)
		expectedLocations.Remove(propagationEntry1.UpstreamRepository)
		expectedLocations.Remove(propagationEntry2.UpstreamRepository)
		assert.Equal(t, 0, expectedLocations.Len())
		expectedUpstreamIDs := set.NewSetFromItems(upstreamEntry1.GetID().String(), upstreamEntry2.GetID().String())
		expectedUpstreamIDs.Remove(propagationEntry1.UpstreamEntryID.String())
		expectedUpstreamIDs.Remove(propagationEntry2.UpstreamEntryID.String())
		assert.Equal(t, 0, expectedUpstreamIDs.Len())

		downstreamRootTreeID, err = downstreamRepo.r.GetCommitTreeID(propagationEntry2.TargetID)
		if err != nil {
			t.Fatal(err)
		}
		pathTree1ID, err := downstreamRepo.r.GetPathIDInTree(localPath1, downstreamRootTreeID)
		if err != nil {
			t.Fatal(err)
		}
		pathTree2ID, err := downstreamRepo.r.GetPathIDInTree(localPath2, downstreamRootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		// Check the subtree IDs in downstream repo matches upstream root tree IDs
		assert.Equal(t, upstreamRootTree1ID, pathTree1ID)
		assert.Equal(t, upstreamRootTree2ID, pathTree2ID)

		// Check the downstream tree still contains other items
		expectedRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("a", blobAID),
			gitinterface.NewEntryBlob("foo/b", blobBID),
			gitinterface.NewEntryBlob("upstream1/a", blobAID),
			gitinterface.NewEntryBlob("upstream1/b", blobBID),
			gitinterface.NewEntryBlob("upstream2/c", blobCID),
			gitinterface.NewEntryBlob("upstream2/d", blobDID),
		})
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectedRootTreeID, downstreamRootTreeID)

		// Nothing to propagate, check that a new entry has not been added in the downstreamRepo
		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		assert.Nil(t, err)

		latestEntry, err = rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, propagationEntry2.GetID(), latestEntry.GetID())
	})
}

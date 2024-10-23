// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordRSLEntryForReference(t *testing.T) {
	tempDir := t.TempDir()
	r := gitinterface.CreateTestGitRepository(t, tempDir, false)

	repo := &Repository{r: r}

	treeBuilder := gitinterface.NewTreeBuilder(repo.r)
	emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
	if err != nil {
		t.Fatal(err)
	}
	commitID, err := repo.r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.RecordRSLEntryForReference("refs/heads/main", false); err != nil {
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

	if err := repo.RecordRSLEntryForReference("main", false); err != nil {
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

	err = repo.RecordRSLEntryForReference("main", false)
	assert.Nil(t, err)

	entryT, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	// check that a duplicate entry has not been created
	assert.Equal(t, entry.GetID(), entryT.GetID())

	// Record entry for a different dst ref
	err = repo.RecordRSLEntryForReference("refs/heads/main", false, rslopts.WithOverrideRefName("refs/heads/not-main"))
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
			emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
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

	err := repo.RecordRSLAnnotation([]string{gitinterface.ZeroHash.String()}, false, "test annotation", false)
	assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)

	treeBuilder := gitinterface.NewTreeBuilder(repo.r)
	emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordRSLEntryForReference("refs/heads/main", false); err != nil {
		t.Fatal(err)
	}

	latestEntry, err := rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	entryID := latestEntry.GetID()

	err = repo.RecordRSLAnnotation([]string{entryID.String()}, false, "test annotation", false)
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

	err = repo.RecordRSLAnnotation([]string{entryID.String()}, true, "skip annotation", false)
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
		emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref})
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)

		// Simulate more remote actions
		if _, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
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
		emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref})
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
		emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref})
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
		if err := localRepo.RecordRSLEntryForReference(refName, false); err != nil {
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
		emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref})
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
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions
		if _, err := localRepo.r.Commit(emptyTreeHash, anotherRefName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(anotherRefName, false); err != nil {
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
		emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref})
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
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions -- NOT anotherRefname here
		if _, err := localRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(refName, false); err != nil {
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
func TestCheckRemoteRSLForUpdates(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	t.Run("remote has updates for local", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref})
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

		// Simulate more remote actions
		if _, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has updates
		hasUpdates, hasDiverged, err := localRepo.CheckRemoteRSLForUpdates(testCtx, remoteName)
		assert.Nil(t, err)
		assert.True(t, hasUpdates)
		assert.False(t, hasDiverged)
	})

	t.Run("remote has no updates for local", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref})
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

		// Local should be notified that remote has no updates
		hasUpdates, hasDiverged, err := localRepo.CheckRemoteRSLForUpdates(testCtx, remoteName)
		assert.Nil(t, err)
		assert.False(t, hasUpdates)
		assert.False(t, hasDiverged)
	})

	t.Run("local is ahead of remote", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref})
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
		if err := localRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has no updates
		hasUpdates, hasDiverged, err := localRepo.CheckRemoteRSLForUpdates(testCtx, remoteName)
		assert.Nil(t, err)
		assert.False(t, hasUpdates)
		assert.False(t, hasDiverged)
	})

	t.Run("remote and local have diverged", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate remote actions
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref})
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
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions
		if _, err := localRepo.r.Commit(emptyTreeHash, anotherRefName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(anotherRefName, false); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has updates that needs to be
		// reconciled
		hasUpdates, hasDiverged, err := localRepo.CheckRemoteRSLForUpdates(testCtx, remoteName)
		assert.Nil(t, err)
		assert.True(t, hasUpdates)
		assert.True(t, hasDiverged)
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

func TestGetRSLEntryLog(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	entries, annotationMap, err := GetRSLEntryLog(r)
	assert.Nil(t, err)

	firstEntry, _, err := rsl.GetFirstEntry(r.r)
	if err != nil {
		t.Fatal(err)
	}

	lastEntry, err := rsl.GetLatestEntry(r.r)
	if err != nil {
		t.Fatal(err)
	}

	expected, _, err := rsl.GetReferenceEntriesInRange(r.r, firstEntry.GetID(), lastEntry.GetID())
	if err != nil {
		t.Fatal(err)
	}

	slices.Reverse(expected)
	assert.Equal(t, expected, entries)
	assert.Equal(t, map[string][]*rsl.AnnotationEntry{}, annotationMap)
}

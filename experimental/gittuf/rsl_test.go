// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/gitobject"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/rsl"
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

	customFields := map[string]string{"custom.example.com/source": "forge"}
	if err := repo.RecordRSLEntryForReference(testCtx, "refs/heads/main", false, rslopts.WithRecordLocalOnly(), rslopts.WithRecordCustomFields(customFields)); err != nil {
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
	assert.Equal(t, customFields, map[string]string(entry.CustomFields))

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

	err = repo.RecordRSLEntryForReference(testCtx, "refs/heads/main", false)
	assert.ErrorIs(t, err, ErrRemoteNotSpecified)

	err = repo.RecordRSLEntryForReference(testCtx, "refs/heads/main", false, rslopts.WithRecordRemote("origin"), rslopts.WithRecordLocalOnly())
	assert.ErrorIs(t, err, ErrCannotUseRemoteAndLocalOnly)

	t.Run("sync with remote", func(t *testing.T) {
		remoteName := "origin"
		refName := "refs/heads/main"

		remoteTmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, remoteTmpDir, true)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := remoteR.Commit(emptyTreeHash, refName, "Initial commit\n", false); err != nil {
			t.Fatal(err)
		}
		err = remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly())
		assert.Nil(t, err)

		localTmpDir := t.TempDir()
		localR, err := gitinterface.CloneAndFetchRepository(remoteTmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		localRepo := &Repository{r: localR}

		blobID, err := localR.WriteBlob([]byte("test"))
		if err != nil {
			t.Fatal(err)
		}
		localTreeBuilder := gitinterface.NewTreeBuilder(localR)
		treeID, err := localTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("test.txt", blobID),
		})
		if err != nil {
			t.Fatal(err)
		}
		newCommitID, err := localR.Commit(treeID, refName, "Local commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		err = localRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordRemote(remoteName))
		assert.Nil(t, err)

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, refName)
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)

		entry, _, err := rsl.GetLatestReferenceUpdaterEntry(remoteR, rsl.ForReference(refName))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, refName, entry.GetRefName())
		assert.Equal(t, newCommitID, entry.GetTargetID())
	})

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.RecordRSLEntryForReference(testCtx, "refs/heads/main", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)
	})
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

	customFields := map[string]string{"custom.example.com/source": "forge"}
	err = repo.RecordRSLAnnotation(testCtx, []string{entryID.String()}, false, "test annotation", false, rslopts.WithAnnotateLocalOnly(), rslopts.WithAnnotateCustomFields(customFields))
	assert.Nil(t, err)

	latestEntry, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	assert.IsType(t, &rsl.AnnotationEntry{}, latestEntry)

	annotation := latestEntry.(*rsl.AnnotationEntry)
	assert.Equal(t, "test annotation", annotation.Message)
	assert.Equal(t, []githash.Hash{entryID}, annotation.RSLEntryIDs)
	assert.False(t, annotation.Skip)
	assert.Equal(t, customFields, map[string]string(annotation.CustomFields))

	err = repo.RecordRSLAnnotation(testCtx, []string{entryID.String()}, true, "skip annotation", false, rslopts.WithAnnotateLocalOnly())
	assert.Nil(t, err)

	latestEntry, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	assert.IsType(t, &rsl.AnnotationEntry{}, latestEntry)

	annotation = latestEntry.(*rsl.AnnotationEntry)
	assert.Equal(t, "skip annotation", annotation.Message)
	assert.Equal(t, []githash.Hash{entryID}, annotation.RSLEntryIDs)
	assert.True(t, annotation.Skip)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.RecordRSLAnnotation(testCtx, []string{entryID.String()}, true, "skip annotation", true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)
	})
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

	t.Run("remote uses gittuf transport prefix", func(t *testing.T) {
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
		if err := localR.RemoveRemote(remoteName); err != nil {
			t.Fatal(err)
		}
		if err := localR.AddRemote(remoteName, gittufTransportPrefix+tmpDir); err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

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

		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)
		assert.NotEqual(t, originalRSLTip, currentRSLTip)

		_, err = localR.GetRemoteURL(fmt.Sprintf("check-remote-%s", remoteName))
		assert.ErrorContains(t, err, "No such remote")

		remoteURL, err := localR.GetRemoteURL(remoteName)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, gittufTransportPrefix+tmpDir, remoteURL)
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

	t.Run("miscellaneous error checking", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)
		nr := &Repository{r: repo}

		// Test signCommit
		err := repo.SetGitConfig("user.signingkey", "")
		if err != nil {
			t.Fatal(err)
		}

		err = nr.ReconcileLocalRSLWithRemote(testCtx, remoteName, true)
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)
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

	t.Run("remote uses gittuf transport prefix", func(t *testing.T) {
		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, true)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := remoteR.Commit(emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		defer os.RemoveAll(localTmpDir) //nolint:errcheck
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.Nil(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
		if err := localR.RemoveRemote(remoteName); err != nil {
			t.Fatal(err)
		}
		if err := localR.AddRemote(remoteName, gittufTransportPrefix+tmpDir); err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

		divergedRefs, err := localRepo.Sync(testCtx, remoteName, false, false)
		assert.Nil(t, err)
		assert.Empty(t, divergedRefs)

		_, err = localR.GetRemoteURL(fmt.Sprintf("check-remote-%s", remoteName))
		assert.ErrorContains(t, err, "No such remote")

		remoteURL, err := localR.GetRemoteURL(remoteName)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, gittufTransportPrefix+tmpDir, remoteURL)
		assertLocalAndRemoteRefsMatch(t, localR, remoteR, rsl.Ref)
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

		downstreamRootTreeID, err = downstreamRepo.r.GetCommitTreeID(githash.Hash(propagationEntry.TargetID.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		pathTreeID, err := downstreamRepo.r.GetPathIDInTree(downstreamRootTreeID, localPath)
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

		downstreamRootTreeID, err = downstreamRepo.r.GetCommitTreeID(githash.Hash(propagationEntry2.TargetID.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		pathTree1ID, err := downstreamRepo.r.GetPathIDInTree(downstreamRootTreeID, localPath1)
		if err != nil {
			t.Fatal(err)
		}
		pathTree2ID, err := downstreamRepo.r.GetPathIDInTree(downstreamRootTreeID, localPath2)
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

		downstreamRootTreeID, err = downstreamRepo.r.GetCommitTreeID(githash.Hash(propagationEntry2.TargetID.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		pathTreeID, err := downstreamRepo.r.GetPathIDInTree(downstreamRootTreeID, localPath)
		if err != nil {
			t.Fatal(err)
		}

		// Check the subtree ID in downstream repo matches upstream root tree ID
		assert.Equal(t, upstreamRootTreeID, pathTreeID)
		// Check the tree as a whole is as expected
		assert.Equal(t, expectedRootTreeID, downstreamRootTreeID)

		// Do the same thing for the other propagation entry's tree (this is a different ref!)
		downstreamRootTreeID, err = downstreamRepo.r.GetCommitTreeID(githash.Hash(propagationEntry1.TargetID.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		pathTreeID, err = downstreamRepo.r.GetPathIDInTree(downstreamRootTreeID, localPath)
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

		downstreamRootTreeID, err = downstreamRepo.r.GetCommitTreeID(githash.Hash(propagationEntry2.TargetID.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		pathTree1ID, err := downstreamRepo.r.GetPathIDInTree(downstreamRootTreeID, localPath1)
		if err != nil {
			t.Fatal(err)
		}
		pathTree2ID, err := downstreamRepo.r.GetPathIDInTree(downstreamRootTreeID, localPath2)
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

	t.Run("controller repository metadata is propagated", func(t *testing.T) {
		controllerRepoLocation := t.TempDir()
		controllerRepo := createTestRepositoryWithRoot(t, controllerRepoLocation)

		downstreamRepoLocation := t.TempDir()
		downstreamRepo := createTestRepositoryWithRoot(t, downstreamRepoLocation)

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		initialRootPrincipals := []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())}
		err := downstreamRepo.AddControllerRepository(testCtx, signer, "controller", controllerRepoLocation, initialRootPrincipals, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.StagePolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.ApplyPolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}

		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		if err != nil {
			t.Fatal(err)
		}

		latestEntry, err := rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}

		propagationEntry, isPropagationEntry := latestEntry.(*rsl.PropagationEntry)
		if !isPropagationEntry {
			t.Fatal("unexpected entry type in downstream repo")
		}
		assert.Equal(t, controllerRepoLocation, propagationEntry.UpstreamRepository)
		assert.Equal(t, policy.PolicyRef, propagationEntry.RefName)

		controllerPolicyEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(controllerRepo.r, rsl.ForReference(policy.PolicyRef), rsl.IsUnskipped())
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, controllerPolicyEntry.GetID(), propagationEntry.UpstreamEntryID)

		controllerPolicyTreeID, err := controllerRepo.r.GetCommitTreeID(githash.Hash(controllerPolicyEntry.GetTargetID().Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		controllerMetadataTreeID, err := controllerRepo.r.GetPathIDInTree(controllerPolicyTreeID, "metadata")
		if err != nil {
			t.Fatal(err)
		}

		downstreamPolicyEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(downstreamRepo.r, rsl.ForReference(policy.PolicyRef), rsl.IsUnskipped())
		if err != nil {
			t.Fatal(err)
		}
		downstreamPolicyTreeID, err := downstreamRepo.r.GetCommitTreeID(githash.Hash(downstreamPolicyEntry.GetTargetID().Bytes()))
		if err != nil {
			t.Fatal(err)
		}

		encodedLocation := base64.URLEncoding.EncodeToString([]byte(controllerRepoLocation))
		controllerPath := fmt.Sprintf("%s/controller-%s", tuf.GittufControllerPrefix, encodedLocation)
		propagatedControllerTreeID, err := downstreamRepo.r.GetPathIDInTree(downstreamPolicyTreeID, controllerPath)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, controllerMetadataTreeID, propagatedControllerTreeID)
	})

	t.Run("transitive controller repository metadata is resolved and propagated", func(t *testing.T) {
		leafControllerRepoLocation := t.TempDir()
		leafControllerRepo := createTestRepositoryWithRoot(t, leafControllerRepoLocation)

		directControllerRepoLocation := t.TempDir()
		directControllerRepo := createTestRepositoryWithRoot(t, directControllerRepoLocation)

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
		initialRootPrincipals := []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())}
		err := directControllerRepo.AddControllerRepository(testCtx, signer, "leaf-controller", leafControllerRepoLocation, initialRootPrincipals, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := directControllerRepo.StagePolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}
		if err := directControllerRepo.ApplyPolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}

		downstreamRepoLocation := t.TempDir()
		downstreamRepo := createTestRepositoryWithRoot(t, downstreamRepoLocation)

		err = downstreamRepo.AddControllerRepository(testCtx, signer, "direct-controller", directControllerRepoLocation, initialRootPrincipals, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.StagePolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}
		if err := downstreamRepo.ApplyPolicy(testCtx, "", true, false); err != nil {
			t.Fatal(err)
		}

		previousLatestEntry, err := rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}

		err = downstreamRepo.PropagateChangesFromUpstreamRepositories(testCtx, false)
		if err != nil {
			t.Fatal(err)
		}

		latestEntry, err := rsl.GetLatestEntry(downstreamRepo.r)
		if err != nil {
			t.Fatal(err)
		}

		propagationEntries := []*rsl.PropagationEntry{}
		for !latestEntry.GetID().Equal(previousLatestEntry.GetID().Bytes()) {
			propagationEntry, isPropagationEntry := latestEntry.(*rsl.PropagationEntry)
			if !isPropagationEntry {
				t.Fatal("unexpected entry type in downstream repo")
			}
			propagationEntries = append(propagationEntries, propagationEntry)

			latestEntry, err = rsl.GetParentForEntry(downstreamRepo.r, latestEntry)
			if err != nil {
				t.Fatal(err)
			}
		}

		assert.Equal(t, 2, len(propagationEntries))
		expectedLocations := set.NewSetFromItems(directControllerRepoLocation, leafControllerRepoLocation)
		for _, propagationEntry := range propagationEntries {
			expectedLocations.Remove(propagationEntry.UpstreamRepository)
			assert.Equal(t, policy.PolicyRef, propagationEntry.RefName)
		}
		assert.Equal(t, 0, expectedLocations.Len())

		downstreamPolicyEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(downstreamRepo.r, rsl.ForReference(policy.PolicyRef), rsl.IsUnskipped())
		if err != nil {
			t.Fatal(err)
		}
		downstreamPolicyTreeID, err := downstreamRepo.r.GetCommitTreeID(githash.Hash(downstreamPolicyEntry.GetTargetID().Bytes()))
		if err != nil {
			t.Fatal(err)
		}

		encodedDirectControllerLocation := base64.URLEncoding.EncodeToString([]byte(directControllerRepoLocation))
		directControllerPath := fmt.Sprintf("%s/direct-controller-%s", tuf.GittufControllerPrefix, encodedDirectControllerLocation)
		_, err = downstreamRepo.r.GetPathIDInTree(downstreamPolicyTreeID, directControllerPath)
		if err != nil {
			t.Fatal(err)
		}

		encodedLeafControllerLocation := base64.URLEncoding.EncodeToString([]byte(leafControllerRepoLocation))
		leafControllerPath := fmt.Sprintf("%s/leaf-controller-%s", tuf.GittufControllerPrefix, encodedLeafControllerLocation)
		propagatedLeafControllerTreeID, err := downstreamRepo.r.GetPathIDInTree(downstreamPolicyTreeID, leafControllerPath)
		if err != nil {
			t.Fatal(err)
		}

		leafControllerPolicyEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(leafControllerRepo.r, rsl.ForReference(policy.PolicyRef), rsl.IsUnskipped())
		if err != nil {
			t.Fatal(err)
		}
		leafControllerPolicyTreeID, err := leafControllerRepo.r.GetCommitTreeID(githash.Hash(leafControllerPolicyEntry.GetTargetID().Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		leafControllerMetadataTreeID, err := leafControllerRepo.r.GetPathIDInTree(leafControllerPolicyTreeID, "metadata")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, leafControllerMetadataTreeID, propagatedLeafControllerTreeID)
	})
}

func TestRecordRSLEntryForReferenceWithSigningKeyBytes(t *testing.T) {
	// signCommit=true with no per-repo signing config would normally fail in
	// CanSign(). WithRecordSigningKeyBytes skips CanSign and signs the RSL
	// entry commit with the supplied PEM key directly, the same path
	// CommitUsingSpecificKey takes.
	tempDir := t.TempDir()
	r := gitinterface.CreateTestGitRepository(t, tempDir, false)
	repo := &Repository{r: r}

	treeBuilder := gitinterface.NewTreeBuilder(repo.r)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	require.NoError(t, err)
	_, err = repo.r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
	require.NoError(t, err)

	err = repo.RecordRSLEntryForReference(
		testCtx,
		"refs/heads/main",
		true,
		rslopts.WithRecordLocalOnly(),
		rslopts.WithRecordSigningKeyBytes(artifacts.SSHED25519Private),
	)
	require.NoError(t, err)

	entryT, err := rsl.GetLatestEntry(repo.r)
	require.NoError(t, err)
	entry, ok := entryT.(*rsl.ReferenceEntry)
	require.True(t, ok)
	assert.Equal(t, "refs/heads/main", entry.RefName)

	keyPath := filepath.Join(tempDir, "ssh-key.pub")
	require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519PublicSSH, 0o600))
	publicKey, err := ssh.NewKeyFromFile(keyPath)
	require.NoError(t, err)

	payload, signature, err := repo.r.GetObjectSignature(entry.GetID())
	require.NoError(t, err)
	require.NoError(t, gitobject.Verify(testCtx, publicKey, payload, signature))
}

func TestRecordRSLAnnotationWithSigningKeyBytes(t *testing.T) {
	// Mirror the entry test for annotations.
	tempDir := t.TempDir()
	r := gitinterface.CreateTestGitRepository(t, tempDir, false)
	repo := &Repository{r: r}

	treeBuilder := gitinterface.NewTreeBuilder(repo.r)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	require.NoError(t, err)
	_, err = repo.r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
	require.NoError(t, err)

	require.NoError(t, repo.RecordRSLEntryForReference(
		testCtx,
		"refs/heads/main",
		false,
		rslopts.WithRecordLocalOnly(),
	))
	entryT, err := rsl.GetLatestEntry(repo.r)
	require.NoError(t, err)
	entryID := entryT.GetID()

	err = repo.RecordRSLAnnotation(
		testCtx,
		[]string{entryID.String()},
		false,
		"test annotation",
		true,
		rslopts.WithAnnotateLocalOnly(),
		rslopts.WithAnnotateSigningKeyBytes(artifacts.SSHED25519Private),
	)
	require.NoError(t, err)

	annT, err := rsl.GetLatestEntry(repo.r)
	require.NoError(t, err)
	ann, ok := annT.(*rsl.AnnotationEntry)
	require.True(t, ok)
	assert.Equal(t, "test annotation", ann.Message)

	keyPath := filepath.Join(tempDir, "ssh-key.pub")
	require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519PublicSSH, 0o600))
	publicKey, err := ssh.NewKeyFromFile(keyPath)
	require.NoError(t, err)

	payload, signature, err := repo.r.GetObjectSignature(ann.GetID())
	require.NoError(t, err)
	require.NoError(t, gitobject.Verify(testCtx, publicKey, payload, signature))
}

func TestRecordRSLEntryForReferenceSigningKeyBytesIgnoredWhenUnsigned(t *testing.T) {
	// signCommit=false is the master switch: passing SigningKeyBytes alongside
	// it leaves the entry unsigned. This keeps the API contract aligned with
	// the existing flag rather than letting key presence silently override it.
	tempDir := t.TempDir()
	r := gitinterface.CreateTestGitRepository(t, tempDir, false)
	repo := &Repository{r: r}

	treeBuilder := gitinterface.NewTreeBuilder(repo.r)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	require.NoError(t, err)
	_, err = repo.r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
	require.NoError(t, err)

	require.NoError(t, repo.RecordRSLEntryForReference(
		testCtx,
		"refs/heads/main",
		false,
		rslopts.WithRecordLocalOnly(),
		rslopts.WithRecordSigningKeyBytes(artifacts.SSHED25519Private),
	))

	entryT, err := rsl.GetLatestEntry(repo.r)
	require.NoError(t, err)
	entry, ok := entryT.(*rsl.ReferenceEntry)
	require.True(t, ok)

	keyPath := filepath.Join(tempDir, "ssh-key.pub")
	require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519PublicSSH, 0o600))
	publicKey, err := ssh.NewKeyFromFile(keyPath)
	require.NoError(t, err)

	payload, signature, err := repo.r.GetObjectSignature(entry.GetID())
	require.NoError(t, err)
	err = gitobject.Verify(testCtx, publicKey, payload, signature)
	assert.Error(t, err, "entry must not be signed when signCommit=false")
}

func TestReconcileLocalRSLWithRemoteBulkAndAnnotation(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"

	tmpDir := t.TempDir()
	remoteRepo := createTestRepositoryWithPolicy(t, tmpDir)
	remoteR := remoteRepo.r

	treeBuilder := gitinterface.NewTreeBuilder(remoteR)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	require.NoError(t, err)

	_, err = remoteR.Commit(emptyTreeHash, refName, "Test commit", false)
	require.NoError(t, err)
	require.NoError(t, remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()))

	localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
	defer os.RemoveAll(localTmpDir) //nolint:errcheck
	localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref, policy.PolicyRef}, true)
	require.NoError(t, err)
	require.NoError(t, localR.SetGitConfig("user.name", "Jane Doe"))
	require.NoError(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))
	localRepo := &Repository{r: localR}

	// The entry both RSLs share, recorded before they diverge. It is not
	// replayed, so annotations that refer to it keep its ID.
	sharedEntry, err := rsl.GetLatestEntry(localR)
	require.NoError(t, err)
	sharedEntryID := sharedEntry.GetID()

	_, err = remoteR.Commit(emptyTreeHash, refName, "Test commit", false)
	require.NoError(t, err)
	require.NoError(t, remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()))

	featureID, err := localR.Commit(emptyTreeHash, "refs/heads/feature", "Test commit", false)
	require.NoError(t, err)
	releaseID, err := localR.Commit(emptyTreeHash, "refs/heads/release", "Test commit", false)
	require.NoError(t, err)
	updates := []rsl.ReferenceUpdate{
		{RefName: "refs/heads/feature", TargetID: featureID},
		{RefName: "refs/heads/release", TargetID: releaseID},
	}
	require.NoError(t, rsl.NewBulkReferenceEntry(updates).Commit(localR, false))
	originalBulk, err := rsl.GetLatestEntry(localR)
	require.NoError(t, err)
	originalBulkID := originalBulk.GetID()

	downstreamID, err := localR.Commit(emptyTreeHash, "refs/heads/downstream", "Test commit", false)
	require.NoError(t, err)
	require.NoError(t, rsl.NewPropagationEntry("refs/heads/downstream", downstreamID, "https://example.com/upstream", sharedEntryID).Commit(localR, false))

	// The annotation refers to the shared entry and to the bulk entry. Only
	// the bulk entry is replayed, so only its ID is rewritten.
	require.NoError(t, rsl.NewAnnotationEntryWithQualifiers([]githash.Hash{sharedEntryID, originalBulkID}, map[string][]string{originalBulkID.String(): {"refs/heads/feature"}}, true, "rewritten feature").Commit(localR, false))

	require.NoError(t, localRepo.ReconcileLocalRSLWithRemote(testCtx, remoteName, false))

	tip, err := rsl.GetLatestEntry(localR)
	require.NoError(t, err)
	annotation, isAnnotation := tip.(*rsl.AnnotationEntry)
	require.True(t, isAnnotation)

	replayedPropagationT, err := rsl.GetParentForEntry(localR, tip)
	require.NoError(t, err)
	replayedPropagation, isPropagation := replayedPropagationT.(*rsl.PropagationEntry)
	require.True(t, isPropagation)
	assert.Equal(t, "refs/heads/downstream", replayedPropagation.RefName)
	assert.Equal(t, downstreamID, replayedPropagation.TargetID)
	assert.Equal(t, "https://example.com/upstream", replayedPropagation.UpstreamRepository)
	assert.Equal(t, sharedEntryID, replayedPropagation.UpstreamEntryID)

	replayedBulkT, err := rsl.GetParentForEntry(localR, replayedPropagation)
	require.NoError(t, err)
	replayedBulk, isBulk := replayedBulkT.(*rsl.BulkReferenceEntry)
	require.True(t, isBulk)
	assert.NotEqual(t, originalBulkID, replayedBulk.ID)
	assert.Equal(t, updates, replayedBulk.Updates)

	assert.Equal(t, []githash.Hash{sharedEntryID, replayedBulk.ID}, annotation.RSLEntryIDs)
	assert.Equal(t, map[string][]string{replayedBulk.ID.String(): {"refs/heads/feature"}}, annotation.Refs)
	assert.True(t, annotation.Skip)

	remoteTip, err := remoteR.GetReference(rsl.Ref)
	require.NoError(t, err)
	parentOfBulk, err := rsl.GetParentForEntry(localR, replayedBulk)
	require.NoError(t, err)
	assert.Equal(t, remoteTip, parentOfBulk.GetID())
}

func TestGetLatestRefTipsFromRSLEntriesWithBulk(t *testing.T) {
	t.Parallel()

	bulkID, err := gitinterface.NewHash("abcdef12345678900987654321fedcbaabcdef12")
	require.NoError(t, err)
	target, err := gitinterface.NewHash("1111111111111111111111111111111111111111")
	require.NoError(t, err)

	bulk := &rsl.BulkReferenceEntry{ID: bulkID, Updates: []rsl.ReferenceUpdate{
		{RefName: "refs/heads/main", TargetID: target},
		{RefName: "refs/heads/feature", TargetID: target},
	}}

	t.Run("qualified skip", func(t *testing.T) {
		t.Parallel()

		annotation := rsl.NewAnnotationEntryWithQualifiers([]githash.Hash{bulkID}, map[string][]string{bulkID.String(): {"refs/heads/feature"}}, true, "")

		// entries are newest first, as getRSLEntriesUntil returns them
		tips := getLatestRefTipsFromRSLEntries([]rsl.Entry{annotation, bulk})
		assert.Equal(t, map[string]githash.Hash{"refs/heads/main": target}, tips)
	})

	t.Run("unqualified skip", func(t *testing.T) {
		t.Parallel()

		annotation := rsl.NewAnnotationEntry([]githash.Hash{bulkID}, true, "")

		tips := getLatestRefTipsFromRSLEntries([]rsl.Entry{annotation, bulk})
		assert.Equal(t, map[string]githash.Hash{}, tips)
	})
}

func TestRecordRSLEntryForReferences(t *testing.T) {
	setup := func(t *testing.T) (*Repository, githash.Hash, githash.Hash) {
		t.Helper()
		tempDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, tempDir, false)
		repo := &Repository{r: r}

		treeBuilder := gitinterface.NewTreeBuilder(repo.r)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		mainID, err := repo.r.Commit(emptyTreeHash, "refs/heads/main", "main\n", false)
		require.NoError(t, err)
		featureID, err := repo.r.Commit(emptyTreeHash, "refs/heads/feature", "feature\n", false)
		require.NoError(t, err)
		return repo, mainID, featureID
	}

	t.Run("more than one update produces one bulk entry", func(t *testing.T) {
		repo, mainID, featureID := setup(t)

		err := repo.RecordRSLEntryForReferences(testCtx, []ReferenceUpdateRequest{
			{RefName: "refs/heads/main"},
			{RefName: "refs/heads/feature"},
		}, false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		latest, err := rsl.GetLatestEntry(repo.r)
		require.NoError(t, err)
		bulk, isBulk := latest.(*rsl.BulkReferenceEntry)
		require.True(t, isBulk)
		assert.Equal(t, []rsl.ReferenceUpdate{
			{RefName: "refs/heads/main", TargetID: mainID},
			{RefName: "refs/heads/feature", TargetID: featureID},
		}, bulk.Updates)
	})

	t.Run("overridden ref name is the one recorded", func(t *testing.T) {
		repo, mainID, featureID := setup(t)

		err := repo.RecordRSLEntryForReferences(testCtx, []ReferenceUpdateRequest{
			{RefName: "refs/heads/main"},
			{RefName: "feature", RefNameOverride: "refs/heads/feature-remote"},
		}, false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		latest, err := rsl.GetLatestEntry(repo.r)
		require.NoError(t, err)
		bulk, isBulk := latest.(*rsl.BulkReferenceEntry)
		require.True(t, isBulk)
		assert.Equal(t, []rsl.ReferenceUpdate{
			{RefName: "refs/heads/main", TargetID: mainID},
			{RefName: "refs/heads/feature-remote", TargetID: featureID},
		}, bulk.Updates)
	})

	t.Run("single update is never bulk", func(t *testing.T) {
		repo, mainID, _ := setup(t)

		err := repo.RecordRSLEntryForReferences(testCtx, []ReferenceUpdateRequest{{RefName: "refs/heads/main"}}, false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		latest, err := rsl.GetLatestEntry(repo.r)
		require.NoError(t, err)
		entry, isRef := latest.(*rsl.ReferenceEntry)
		require.True(t, isRef)
		assert.Equal(t, mainID, entry.TargetID)
		assert.False(t, entry.FromBulkEntry())
	})

	t.Run("repeated recorded ref is collapsed with the last tip", func(t *testing.T) {
		repo, _, _ := setup(t)

		treeBuilder := gitinterface.NewTreeBuilder(repo.r)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		laterMainID, err := repo.r.Commit(emptyTreeHash, "refs/heads/main", "main again\n", false)
		require.NoError(t, err)
		featureID, err := repo.r.GetReference("refs/heads/feature")
		require.NoError(t, err)

		err = repo.RecordRSLEntryForReferences(testCtx, []ReferenceUpdateRequest{
			{RefName: "refs/heads/main"},
			{RefName: "refs/heads/feature"},
			{RefName: "main", RefNameOverride: "refs/heads/main"},
		}, false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		latest, err := rsl.GetLatestEntry(repo.r)
		require.NoError(t, err)
		bulk, isBulk := latest.(*rsl.BulkReferenceEntry)
		require.True(t, isBulk)
		assert.Equal(t, []rsl.ReferenceUpdate{
			{RefName: "refs/heads/main", TargetID: laterMainID},
			{RefName: "refs/heads/feature", TargetID: featureID},
		}, bulk.Updates)
	})

	t.Run("filtering runs after collapsing repeated refs", func(t *testing.T) {
		repo, mainID, featureID := setup(t)

		// The RSL records main at its first tip, which is also the tip the
		// first request for main in the batch resolves to.
		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, "refs/heads/main", false, rslopts.WithRecordLocalOnly()))
		require.NoError(t, repo.r.SetReference("refs/heads/earlier-main", mainID))

		treeBuilder := gitinterface.NewTreeBuilder(repo.r)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		laterMainID, err := repo.r.Commit(emptyTreeHash, "refs/heads/main", "main again\n", false)
		require.NoError(t, err)

		err = repo.RecordRSLEntryForReferences(testCtx, []ReferenceUpdateRequest{
			{RefName: "refs/heads/earlier-main", RefNameOverride: "refs/heads/main"},
			{RefName: "refs/heads/feature"},
			{RefName: "refs/heads/main"},
		}, false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		latest, err := rsl.GetLatestEntry(repo.r)
		require.NoError(t, err)
		bulk, isBulk := latest.(*rsl.BulkReferenceEntry)
		require.True(t, isBulk)
		assert.Equal(t, []rsl.ReferenceUpdate{
			{RefName: "refs/heads/main", TargetID: laterMainID},
			{RefName: "refs/heads/feature", TargetID: featureID},
		}, bulk.Updates)
	})

	t.Run("one remaining update after filtering is not a bulk entry", func(t *testing.T) {
		repo, mainID, featureID := setup(t)

		require.NoError(t, repo.RecordRSLEntryForReference(testCtx, "refs/heads/main", false, rslopts.WithRecordLocalOnly()))

		err := repo.RecordRSLEntryForReferences(testCtx, []ReferenceUpdateRequest{
			{RefName: "refs/heads/main"},
			{RefName: "refs/heads/feature"},
		}, false, rslopts.WithRecordLocalOnly())
		require.NoError(t, err)

		latest, err := rsl.GetLatestEntry(repo.r)
		require.NoError(t, err)
		entry, isRef := latest.(*rsl.ReferenceEntry)
		require.True(t, isRef)
		assert.Equal(t, "refs/heads/feature", entry.RefName)
		assert.Equal(t, featureID, entry.TargetID)
		assert.False(t, entry.FromBulkEntry())

		parent, err := rsl.GetParentForEntry(repo.r, latest)
		require.NoError(t, err)
		previous, isRef := parent.(*rsl.ReferenceEntry)
		require.True(t, isRef)
		assert.Equal(t, "refs/heads/main", previous.RefName)
		assert.Equal(t, mainID, previous.TargetID)
	})
}

func TestReconcileLocalRSLWithRemoteDivergenceDetection(t *testing.T) {
	t.Parallel()

	remoteName := "origin"
	refName := "refs/heads/main"

	setup := func(t *testing.T) (*Repository, *Repository, githash.Hash) {
		t.Helper()

		tmpDir := t.TempDir()
		remoteR := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		remoteRepo := &Repository{r: remoteR}

		treeBuilder := gitinterface.NewTreeBuilder(remoteR)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)

		_, err = remoteR.Commit(emptyTreeHash, refName, "Test commit", false)
		require.NoError(t, err)
		require.NoError(t, remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()))

		localTmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("local-%s", t.Name()))
		t.Cleanup(func() {
			os.RemoveAll(localTmpDir) //nolint:errcheck
		})
		localR, err := gitinterface.CloneAndFetchRepository(tmpDir, localTmpDir, refName, []string{rsl.Ref}, true)
		require.NoError(t, err)
		require.NoError(t, localR.SetGitConfig("user.name", "Jane Doe"))
		require.NoError(t, localR.SetGitConfig("user.email", "jane.doe@example.com"))

		// The remote records another entry for refName after the clone, so the
		// two RSLs diverge.
		_, err = remoteR.Commit(emptyTreeHash, refName, "Test commit", false)
		require.NoError(t, err)
		require.NoError(t, remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()))

		return remoteRepo, &Repository{r: localR}, emptyTreeHash
	}

	t.Run("local propagation entry for the ref the remote also updated", func(t *testing.T) {
		t.Parallel()

		remoteRepo, localRepo, emptyTreeHash := setup(t)

		sharedEntry, err := rsl.GetLatestEntry(localRepo.r)
		require.NoError(t, err)

		mainID, err := localRepo.r.Commit(emptyTreeHash, refName, "Test commit", false)
		require.NoError(t, err)
		require.NoError(t, rsl.NewPropagationEntry(refName, mainID, "https://example.com/upstream", sharedEntry.GetID()).Commit(localRepo.r, false))

		originalLocalTip, err := localRepo.r.GetReference(rsl.Ref)
		require.NoError(t, err)
		originalRemoteTip, err := remoteRepo.r.GetReference(rsl.Ref)
		require.NoError(t, err)

		err = localRepo.ReconcileLocalRSLWithRemote(testCtx, remoteName, false)
		assert.ErrorContains(t, err, "changes to the same ref")
		assert.ErrorContains(t, err, refName)

		currentLocalTip, err := localRepo.r.GetReference(rsl.Ref)
		require.NoError(t, err)
		currentRemoteTip, err := remoteRepo.r.GetReference(rsl.Ref)
		require.NoError(t, err)
		assert.Equal(t, originalLocalTip, currentLocalTip)
		assert.Equal(t, originalRemoteTip, currentRemoteTip)
	})

	t.Run("local bulk entry whose second update is for the ref the remote also updated", func(t *testing.T) {
		t.Parallel()

		remoteRepo, localRepo, emptyTreeHash := setup(t)

		featureID, err := localRepo.r.Commit(emptyTreeHash, "refs/heads/feature", "Test commit", false)
		require.NoError(t, err)
		mainID, err := localRepo.r.Commit(emptyTreeHash, refName, "Test commit", false)
		require.NoError(t, err)
		require.NoError(t, rsl.NewBulkReferenceEntry([]rsl.ReferenceUpdate{
			{RefName: "refs/heads/feature", TargetID: featureID},
			{RefName: refName, TargetID: mainID},
		}).Commit(localRepo.r, false))

		originalLocalTip, err := localRepo.r.GetReference(rsl.Ref)
		require.NoError(t, err)
		originalRemoteTip, err := remoteRepo.r.GetReference(rsl.Ref)
		require.NoError(t, err)

		err = localRepo.ReconcileLocalRSLWithRemote(testCtx, remoteName, false)
		assert.ErrorContains(t, err, "changes to the same ref")
		assert.ErrorContains(t, err, refName)

		currentLocalTip, err := localRepo.r.GetReference(rsl.Ref)
		require.NoError(t, err)
		currentRemoteTip, err := remoteRepo.r.GetReference(rsl.Ref)
		require.NoError(t, err)
		assert.Equal(t, originalLocalTip, currentLocalTip)
		assert.Equal(t, originalRemoteTip, currentRemoteTip)
	})

	t.Run("local bulk entry for refs the remote did not update", func(t *testing.T) {
		t.Parallel()

		remoteRepo, localRepo, emptyTreeHash := setup(t)

		featureID, err := localRepo.r.Commit(emptyTreeHash, "refs/heads/feature", "Test commit", false)
		require.NoError(t, err)
		releaseID, err := localRepo.r.Commit(emptyTreeHash, "refs/heads/release", "Test commit", false)
		require.NoError(t, err)
		updates := []rsl.ReferenceUpdate{
			{RefName: "refs/heads/feature", TargetID: featureID},
			{RefName: "refs/heads/release", TargetID: releaseID},
		}
		require.NoError(t, rsl.NewBulkReferenceEntry(updates).Commit(localRepo.r, false))

		originalBulk, err := rsl.GetLatestEntry(localRepo.r)
		require.NoError(t, err)
		originalRemoteTip, err := remoteRepo.r.GetReference(rsl.Ref)
		require.NoError(t, err)

		require.NoError(t, localRepo.ReconcileLocalRSLWithRemote(testCtx, remoteName, false))

		tip, err := rsl.GetLatestEntry(localRepo.r)
		require.NoError(t, err)
		replayedBulk, isBulk := tip.(*rsl.BulkReferenceEntry)
		require.True(t, isBulk)
		assert.NotEqual(t, originalBulk.GetID(), replayedBulk.ID)
		assert.Equal(t, updates, replayedBulk.Updates)

		parentOfBulk, err := rsl.GetParentForEntry(localRepo.r, replayedBulk)
		require.NoError(t, err)
		assert.Equal(t, originalRemoteTip, parentOfBulk.GetID())

		currentRemoteTip, err := remoteRepo.r.GetReference(rsl.Ref)
		require.NoError(t, err)
		assert.Equal(t, originalRemoteTip, currentRemoteTip)
	})
}

// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/eval"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestRecordRSLEntryForReference(t *testing.T) {
	r, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	repo := &Repository{r: r}

	if err := rsl.InitializeNamespace(repo.r); err != nil {
		t.Fatal(err)
	}

	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.ZeroHash)

	if err := repo.r.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}

	if err := repo.RecordRSLEntryForReference("refs/heads/main", false); err != nil {
		t.Fatal(err)
	}

	rslRef, err := repo.r.Reference(rsl.Ref, true)
	if err != nil {
		t.Fatal(err)
	}

	entryType, err := rsl.GetEntry(repo.r, rslRef.Hash())
	if err != nil {
		t.Fatal(err)
	}

	entry, ok := entryType.(*rsl.ReferenceEntry)
	if !ok {
		t.Fatal(fmt.Errorf("invalid entry type"))
	}
	assert.Equal(t, "refs/heads/main", entry.RefName)
	assert.Equal(t, plumbing.ZeroHash, entry.TargetID)

	testHash := plumbing.NewHash("abcdef1234567890")

	ref = plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), testHash)
	if err := repo.r.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}

	if err := repo.RecordRSLEntryForReference("main", false); err != nil {
		t.Fatal(err)
	}

	rslRef, err = repo.r.Reference(rsl.Ref, true)
	if err != nil {
		t.Fatal(err)
	}

	entryType, err = rsl.GetEntry(repo.r, rslRef.Hash())
	if err != nil {
		t.Fatal(err)
	}

	entry, ok = entryType.(*rsl.ReferenceEntry)
	if !ok {
		t.Fatal(fmt.Errorf("invalid entry type"))
	}
	assert.Equal(t, "refs/heads/main", entry.RefName)
	assert.Equal(t, testHash, entry.TargetID)

	err = repo.RecordRSLEntryForReference("main", false)
	assert.Nil(t, err)

	rslRef, err = repo.r.Reference(rsl.Ref, true)
	if err != nil {
		t.Fatal(err)
	}

	entryType, err = rsl.GetEntry(repo.r, rslRef.Hash())
	if err != nil {
		t.Fatal(err)
	}
	// check that a duplicate entry has not been created
	assert.Equal(t, entry.GetID(), entryType.GetID())
}

func TestRecordRSLEntryForReferenceAtCommit(t *testing.T) {
	t.Setenv(eval.EvalModeKey, "1")

	r, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	repo := &Repository{r: r}

	if err := rsl.InitializeNamespace(repo.r); err != nil {
		t.Fatal(err)
	}

	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"
	emptyTreeHash, err := gitinterface.WriteTree(repo.r, nil)
	if err != nil {
		t.Fatal(err)
	}
	commitID, err := gitinterface.Commit(repo.r, emptyTreeHash, refName, "Test commit", false)
	if err != nil {
		t.Fatal(err)
	}

	err = repo.RecordRSLEntryForReferenceAtCommit(refName, commitID.String(), false)
	assert.Nil(t, err)

	latestEntry, err := rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, refName, latestEntry.(*rsl.ReferenceEntry).RefName)
	assert.Equal(t, commitID, latestEntry.(*rsl.ReferenceEntry).TargetID)

	// Now checkout another branch, add another commit
	if err := repo.r.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(anotherRefName), commitID)); err != nil {
		t.Fatal(err)
	}
	newCommitID, err := gitinterface.Commit(repo.r, emptyTreeHash, anotherRefName, "Commit on feature branch", false)
	if err != nil {
		t.Fatal(err)
	}

	err = repo.RecordRSLEntryForReferenceAtCommit(refName, newCommitID.String(), false)
	assert.ErrorIs(t, err, ErrCommitNotInRef)

	// We can, however, record an RSL entry for the commit in the new branch
	err = repo.RecordRSLEntryForReferenceAtCommit(anotherRefName, newCommitID.String(), false)
	assert.Nil(t, err)

	// Finally, let's record a couple more commits and use the older of the two
	commitID, err = gitinterface.Commit(repo.r, emptyTreeHash, refName, "Another commit", false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = gitinterface.Commit(repo.r, emptyTreeHash, refName, "Latest commit", false)
	if err != nil {
		t.Fatal(err)
	}

	err = repo.RecordRSLEntryForReferenceAtCommit(refName, commitID.String(), false)
	assert.Nil(t, err)

	latestEntry, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	latestEntryID := latestEntry.GetID()

	// Now try and duplicate that
	err = repo.RecordRSLEntryForReferenceAtCommit(refName, commitID.String(), false)
	assert.Nil(t, err)

	latestEntry, err = rsl.GetLatestEntry(repo.r)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, latestEntryID, latestEntry.GetID())
}

func TestRecordRSLAnnotation(t *testing.T) {
	r, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	repo := &Repository{r: r}

	if err := rsl.InitializeNamespace(repo.r); err != nil {
		t.Fatal(err)
	}

	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.ZeroHash)

	if err := repo.r.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}

	err = repo.RecordRSLAnnotation([]string{plumbing.ZeroHash.String()}, false, "test annotation", false)
	assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)

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
	assert.Equal(t, []plumbing.Hash{entryID}, annotation.RSLEntryIDs)
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
	assert.Equal(t, []plumbing.Hash{entryID}, annotation.RSLEntryIDs)
	assert.True(t, annotation.Skip)
}

func TestCheckRemoteRSLForUpdates(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	t.Run("remote has updates for local", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		// Simulate remote actions
		remoteR, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatal(err)
		}
		remoteRepo := &Repository{r: remoteR}

		// We can't use remoteRepo.InitializeNamespaces() as it'll create zero
		// namespace for policy, an issue when syncing.
		if err := rsl.InitializeNamespace(remoteRepo.r); err != nil {
			t.Fatal(err)
		}

		if _, err := gitinterface.Commit(remoteRepo.r, gitinterface.EmptyTree(), refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localR, err := gitinterface.CloneAndFetchToMemory(context.Background(), tmpDir, refName, []string{rsl.Ref})
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

		// Simulate more remote actions
		if _, err := gitinterface.Commit(remoteRepo.r, gitinterface.EmptyTree(), refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has updates
		hasUpdates, hasDiverged, err := localRepo.CheckRemoteRSLForUpdates(context.Background(), remoteName)
		assert.Nil(t, err)
		assert.True(t, hasUpdates)
		assert.False(t, hasDiverged)
	})

	t.Run("remote has no updates for local", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		// Simulate remote actions
		remoteR, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatal(err)
		}
		remoteRepo := &Repository{r: remoteR}

		// We can't use remoteRepo.InitializeNamespaces() as it'll create zero
		// namespace for policy, an issue when syncing.
		if err := rsl.InitializeNamespace(remoteRepo.r); err != nil {
			t.Fatal(err)
		}

		if _, err := gitinterface.Commit(remoteRepo.r, gitinterface.EmptyTree(), refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localR, err := gitinterface.CloneAndFetchToMemory(context.Background(), tmpDir, refName, []string{rsl.Ref})
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

		// Local should be notified that remote has no updates
		hasUpdates, hasDiverged, err := localRepo.CheckRemoteRSLForUpdates(context.Background(), remoteName)
		assert.Nil(t, err)
		assert.False(t, hasUpdates)
		assert.False(t, hasDiverged)
	})

	t.Run("local is ahead of remote", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		// Simulate remote actions
		remoteR, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatal(err)
		}
		remoteRepo := &Repository{r: remoteR}

		// We can't use remoteRepo.InitializeNamespaces() as it'll create zero
		// namespace for policy, an issue when syncing.
		if err := rsl.InitializeNamespace(remoteRepo.r); err != nil {
			t.Fatal(err)
		}

		if _, err := gitinterface.Commit(remoteRepo.r, gitinterface.EmptyTree(), refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localR, err := gitinterface.CloneAndFetchToMemory(context.Background(), tmpDir, refName, []string{rsl.Ref})
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

		// Simulate local actions
		if _, err := gitinterface.Commit(localRepo.r, gitinterface.EmptyTree(), refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has no updates
		hasUpdates, hasDiverged, err := localRepo.CheckRemoteRSLForUpdates(context.Background(), remoteName)
		assert.Nil(t, err)
		assert.False(t, hasUpdates)
		assert.False(t, hasDiverged)
	})

	t.Run("remote and local have diverged", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		// Simulate remote actions
		remoteR, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatal(err)
		}
		remoteRepo := &Repository{r: remoteR}

		// We can't use remoteRepo.InitializeNamespaces() as it'll create zero
		// namespace for policy, an issue when syncing.
		if err := rsl.InitializeNamespace(remoteRepo.r); err != nil {
			t.Fatal(err)
		}

		if _, err := gitinterface.Commit(remoteRepo.r, gitinterface.EmptyTree(), refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Clone remote repository
		// TODO: this should be handled by the Repository package
		localR, err := gitinterface.CloneAndFetchToMemory(context.Background(), tmpDir, refName, []string{rsl.Ref})
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localR}

		// Simulate remote actions
		if _, err := gitinterface.Commit(remoteRepo.r, gitinterface.EmptyTree(), refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
			t.Fatal(err)
		}

		// Simulate local actions
		if _, err := gitinterface.Commit(localRepo.r, gitinterface.EmptyTree(), anotherRefName, "Test commit", false); err != nil {
			t.Fatal(err)
		}
		if err := localRepo.RecordRSLEntryForReference(anotherRefName, false); err != nil {
			t.Fatal(err)
		}

		// Local should be notified that remote has updates that needs to be
		// reconciled
		hasUpdates, hasDiverged, err := localRepo.CheckRemoteRSLForUpdates(context.Background(), remoteName)
		assert.Nil(t, err)
		assert.True(t, hasUpdates)
		assert.True(t, hasDiverged)
	})
}

func TestPushRSL(t *testing.T) {
	remoteName := "origin"

	t.Run("successful push", func(t *testing.T) {
		remoteTmpDir := t.TempDir()

		remoteRepo, err := git.PlainInit(remoteTmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		localRepo := createTestRepositoryWithPolicy(t, "")
		if _, err := localRepo.r.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{remoteTmpDir},
		}); err != nil {
			t.Fatal(err)
		}

		err = localRepo.PushRSL(context.Background(), remoteName)
		assert.Nil(t, err)

		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, rsl.Ref)

		// No updates, successful push
		err = localRepo.PushRSL(context.Background(), remoteName)
		assert.Nil(t, err)
	})

	t.Run("divergent RSLs, unsuccessful push", func(t *testing.T) {
		remoteTmpDir := t.TempDir()

		remoteRepo, err := git.PlainInit(remoteTmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.InitializeNamespace(remoteRepo); err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewReferenceEntry(policy.PolicyRef, plumbing.ZeroHash).Commit(remoteRepo, false); err != nil {
			t.Fatal(err)
		}

		localRepo := createTestRepositoryWithPolicy(t, "")
		if _, err := localRepo.r.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{remoteTmpDir},
		}); err != nil {
			t.Fatal(err)
		}

		err = localRepo.PushRSL(context.Background(), remoteName)
		assert.ErrorIs(t, err, ErrPushingRSL)
	})
}

func TestPullRSL(t *testing.T) {
	remoteName := "origin"

	t.Run("successful pull", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		remoteRepo := createTestRepositoryWithPolicy(t, remoteTmpDir)

		localRepoR, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localRepoR}
		if _, err := localRepo.r.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{remoteTmpDir},
		}); err != nil {
			t.Fatal(err)
		}

		err = localRepo.PullRSL(context.Background(), remoteName)
		assert.Nil(t, err)

		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo.r, rsl.Ref)

		// No updates, successful pull
		err = localRepo.PullRSL(context.Background(), remoteName)
		assert.Nil(t, err)
	})

	t.Run("divergent RSLs, unsuccessful pull", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		createTestRepositoryWithPolicy(t, remoteTmpDir)

		localRepoR, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}
		localRepo := &Repository{r: localRepoR}

		if err := rsl.InitializeNamespace(localRepo.r); err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewReferenceEntry(policy.PolicyRef, plumbing.ZeroHash).Commit(localRepo.r, false); err != nil {
			t.Fatal(err)
		}

		if _, err := localRepo.r.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{remoteTmpDir},
		}); err != nil {
			t.Fatal(err)
		}

		err = localRepo.PullRSL(context.Background(), remoteName)
		assert.ErrorIs(t, err, ErrPullingRSL)
	})
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/stretchr/testify/assert"
)

func TestPushPolicy(t *testing.T) {
	remoteName := "origin"

	t.Run("successful push", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		remoteRepo := gitinterface.CreateTestGitRepository(t, remoteTmpDir, false)

		localRepo := createTestRepositoryWithPolicy(t, "")

		if err := policy.Apply(testCtx, localRepo.r, false); err != nil {
			t.Fatal(err)
		}

		if err := localRepo.r.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.PushPolicy(remoteName)
		assert.Nil(t, err)

		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, policy.PolicyRef)
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, policy.PolicyStagingRef)
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, rsl.Ref)

		// No updates, successful push
		err = localRepo.PushPolicy(remoteName)
		assert.Nil(t, err)
	})

	t.Run("divergent policies, unsuccessful push", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		remoteRepo := gitinterface.CreateTestGitRepository(t, remoteTmpDir, false)

		if err := rsl.NewReferenceEntry(policy.PolicyRef, gitinterface.ZeroHash).Commit(remoteRepo, false); err != nil {
			t.Fatal(err)
		}

		localRepo := createTestRepositoryWithPolicy(t, "")

		if err := policy.Apply(testCtx, localRepo.r, false); err != nil {
			t.Fatal(err)
		}

		if err := localRepo.r.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.PushPolicy(remoteName)
		assert.ErrorIs(t, err, ErrPushingPolicy)
	})
}

func TestPullPolicy(t *testing.T) {
	remoteName := "origin"

	t.Run("successful pull", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		remoteRepo := createTestRepositoryWithPolicy(t, remoteTmpDir)
		if err := policy.Apply(testCtx, remoteRepo.r, false); err != nil {
			t.Fatal(err)
		}

		localTmpDir := t.TempDir()
		localRepoR := gitinterface.CreateTestGitRepository(t, localTmpDir, false)
		localRepo := &Repository{r: localRepoR}

		if err := localRepo.r.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.PullPolicy(remoteName)
		assert.Nil(t, err)

		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo.r, policy.PolicyRef)
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo.r, policy.PolicyStagingRef)
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo.r, rsl.Ref)

		// No updates, successful push
		err = localRepo.PullPolicy(remoteName)
		assert.Nil(t, err)
	})

	t.Run("divergent policies, unsuccessful pull", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		createTestRepositoryWithPolicy(t, remoteTmpDir)

		localTmpDir := t.TempDir()
		localRepoR := gitinterface.CreateTestGitRepository(t, localTmpDir, false)
		localRepo := &Repository{r: localRepoR}

		if err := rsl.NewReferenceEntry(policy.PolicyRef, gitinterface.ZeroHash).Commit(localRepo.r, false); err != nil {
			t.Fatal(err)
		}

		if err := localRepo.r.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.PullPolicy(remoteName)
		assert.ErrorIs(t, err, ErrPullingPolicy)
	})
}
func TestDiscardPolicy(t *testing.T) {
	t.Run("successful discard with existing policy", func(t *testing.T) {
		repo := createTestRepositoryWithPolicy(t, "")

		if err := policy.Apply(testCtx, repo.r, false); err != nil {
			t.Fatal(err)
		}

		initialPolicyRef, err := repo.r.GetReference(policy.PolicyRef)
		assert.Nil(t, err)

		err = repo.DiscardPolicy()
		assert.Nil(t, err)

		stagingRef, err := repo.r.GetReference(policy.PolicyStagingRef)
		assert.Nil(t, err)
		assert.Equal(t, initialPolicyRef, stagingRef)
	})

	t.Run("discard with no policy references", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repo := &Repository{r: r}

		err := repo.DiscardPolicy()
		assert.Nil(t, err)

		_, err = repo.r.GetReference(policy.PolicyStagingRef)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)
	})

	t.Run("discard after policy changes", func(t *testing.T) {
		repo := createTestRepositoryWithPolicy(t, "")

		initialRef, err := repo.r.GetReference(policy.PolicyRef)
		assert.Nil(t, err)

		if err := repo.r.SetReference(policy.PolicyStagingRef, gitinterface.ZeroHash); err != nil {
			t.Fatal(err)
		}

		err = repo.DiscardPolicy()
		assert.Nil(t, err)

		stagingRef, err := repo.r.GetReference(policy.PolicyStagingRef)
		assert.Nil(t, err)
		assert.Equal(t, initialRef, stagingRef)
	})
}

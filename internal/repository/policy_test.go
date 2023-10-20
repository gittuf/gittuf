// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestPushPolicy(t *testing.T) {
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

		err = localRepo.PushPolicy(context.Background(), remoteName)
		assert.Nil(t, err)

		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, policy.PolicyRef)
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, rsl.Ref)

		// No updates, successful push
		err = localRepo.PushPolicy(context.Background(), remoteName)
		assert.Nil(t, err)
	})

	t.Run("divergent policies, unsuccessful push", func(t *testing.T) {
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

		err = localRepo.PushPolicy(context.Background(), remoteName)
		assert.ErrorIs(t, err, ErrPushingPolicy)
	})
}

func TestPullPolicy(t *testing.T) {
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

		err = localRepo.PullPolicy(context.Background(), remoteName)
		assert.Nil(t, err)

		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo.r, policy.PolicyRef)
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo.r, rsl.Ref)

		// No updates, successful push
		err = localRepo.PullPolicy(context.Background(), remoteName)
		assert.Nil(t, err)
	})

	t.Run("divergent policies, unsuccessful pull", func(t *testing.T) {
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

		err = localRepo.PullPolicy(context.Background(), remoteName)
		assert.ErrorIs(t, err, ErrPullingPolicy)
	})
}

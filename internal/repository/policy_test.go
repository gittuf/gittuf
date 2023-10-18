// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
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

		localRepo := createTestRepositoryWithPolicy(t)
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

		if err := rsl.NewEntry(policy.PolicyRef, plumbing.ZeroHash).Commit(remoteRepo, false); err != nil {
			t.Fatal(err)
		}

		localRepo := createTestRepositoryWithPolicy(t)
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

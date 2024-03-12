// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
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
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, policy.PolicyStagingRef)
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, rsl.Ref)

		// No updates, successful push
		err = localRepo.PushPolicy(context.Background(), remoteName)
		assert.Nil(t, err)
	})

	t.Run("No changes to policy, but changes to policy staging", func(t *testing.T) {
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
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, policy.PolicyStagingRef)
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, rsl.Ref)

		// No updates, successful push
		err = localRepo.PushPolicy(context.Background(), remoteName)
		assert.Nil(t, err)

		// Create changes in policy staging
		newKey, err := tuf.LoadKeyFromBytes(rootPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
		if err != nil {
			t.Fatal(err)
		}

		// Load current staging state
		currentStagingState, err := policy.LoadCurrentState(context.Background(), localRepo.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata, err := currentStagingState.GetRootMetadata()
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata = policy.AddRootKey(rootMetadata, newKey)

		env, err := dsse.CreateEnvelope(rootMetadata)
		if err != nil {
			t.Fatal(err)
		}

		env, err = dsse.SignEnvelope(context.Background(), env, signer)

		if err != nil {
			t.Fatal(err)
		}

		currentStagingState.RootEnvelope = env

		// Commit changes to policy staging
		if err := currentStagingState.Commit(context.Background(), localRepo.r, "Add new key to root", false, policy.PolicyStagingRef); err != nil {
			t.Fatal(err)
		}

		// check that push works, even when no changes have been made to policy
		err = localRepo.PushPolicy(context.Background(), remoteName)
		assert.Nil(t, err)

		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, policy.PolicyRef)
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, policy.PolicyStagingRef)
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo, rsl.Ref)
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
		assertLocalAndRemoteRefsMatch(t, localRepo.r, remoteRepo.r, policy.PolicyStagingRef)
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

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
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

func TestHasPolicy(t *testing.T) {
	t.Run("policy exists", func(t *testing.T) {
		repo := createTestRepositoryWithPolicy(t, "")
		hasPolicy, err := repo.HasPolicy()
		assert.Nil(t, err)
		assert.True(t, hasPolicy)
	})

	t.Run("policy does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repo := &Repository{r: r}

		hasPolicy, err := repo.HasPolicy()
		assert.Nil(t, err)
		assert.False(t, hasPolicy)
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

func TestListRules(t *testing.T) {
	t.Run("no delegations", func(t *testing.T) {
		repo := createTestRepositoryWithPolicyWithFileRule(t, "")

		rules, err := repo.ListRules(context.Background(), policy.PolicyRef)
		assert.Nil(t, err)

		expectedRules := []*DelegationWithDepth{
			{
				Delegation: &tufv02.Delegation{
					Name:        "protect-main",
					Paths:       []string{"git:refs/heads/main"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("157507bbe151e378ce8126c1dcfe043cdd2db96e"),
						Threshold:    1,
					},
				},
				Depth: 0,
			},
			{
				Delegation: &tufv02.Delegation{
					Name:        "protect-files-1-and-2",
					Paths:       []string{"file:1", "file:2"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("157507bbe151e378ce8126c1dcfe043cdd2db96e"),
						Threshold:    1,
					},
				},
				Depth: 0,
			},
		}
		assert.Equal(t, expectedRules, rules)
	})

	t.Run("with delegations", func(t *testing.T) {
		repo := createTestRepositoryWithDelegatedPolicies(t, "")

		rules, err := repo.ListRules(context.Background(), policy.PolicyRef)
		assert.Nil(t, err)

		expectedRules := []*DelegationWithDepth{
			{
				Delegation: &tufv02.Delegation{
					Name:        "protect-file-1",
					Paths:       []string{"file:1"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("SHA256:ESJezAOo+BsiEpddzRXS6+wtF16FID4NCd+3gj96rFo"),
						Threshold:    1,
					},
				},
				Depth: 0,
			},
			{
				Delegation: &tufv02.Delegation{
					Name:        "3",
					Paths:       []string{"file:1/subpath1/*"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("157507bbe151e378ce8126c1dcfe043cdd2db96e"),
						Threshold:    1,
					},
				},
				Depth: 1,
			},
			{
				Delegation: &tufv02.Delegation{
					Name:        "4",
					Paths:       []string{"file:1/subpath2/*"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("157507bbe151e378ce8126c1dcfe043cdd2db96e"),
						Threshold:    1,
					},
				},
				Depth: 1,
			},
			{
				Delegation: &tufv02.Delegation{
					Name:        "1",
					Paths:       []string{"file:1/*"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("SHA256:ESJezAOo+BsiEpddzRXS6+wtF16FID4NCd+3gj96rFo"),
						Threshold:    1,
					},
				},
				Depth: 0,
			},
			{
				Delegation: &tufv02.Delegation{
					Name:        "2",
					Paths:       []string{"file:2/*"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("SHA256:ESJezAOo+BsiEpddzRXS6+wtF16FID4NCd+3gj96rFo"),
						Threshold:    1,
					},
				},
				Depth: 0,
			},
		}
		assert.Equal(t, expectedRules, rules)
	})
}

func TestListPrincipals(t *testing.T) {
	repo := createTestRepositoryWithPolicyWithFileRule(t, "")

	t.Run("policy exists", func(t *testing.T) {
		pubKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}
		pubKey := tufv02.NewKeyFromSSLibKey(pubKeyR)
		expectedPrincipals := map[string]tuf.Principal{pubKey.KeyID: pubKey}

		principals, err := repo.ListPrincipals(context.Background(), policy.PolicyRef, tuf.TargetsRoleName)
		assert.Nil(t, err)
		assert.Equal(t, expectedPrincipals, principals)
	})

	t.Run("policy does not exist", func(t *testing.T) {
		principals, err := repo.ListPrincipals(testCtx, policy.PolicyRef, "does-not-exist")
		assert.ErrorIs(t, err, policy.ErrPolicyNotFound)
		assert.Nil(t, principals)
	})
}

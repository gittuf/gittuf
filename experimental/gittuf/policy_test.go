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
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		// PolicyIndexRef is intentionally local-only — should NOT exist on remote.
		_, err = remoteRepo.GetReference(policy.PolicyIndexRef)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

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

		// PolicyIndexRef is intentionally not fetched — it's local-only.
		_, err = localRepo.r.GetReference(policy.PolicyIndexRef)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

		// No updates, successful pull
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

		stagingRef, err := repo.r.GetReference(policy.PolicyIndexRef)
		assert.Nil(t, err)
		assert.Equal(t, initialPolicyRef, stagingRef)
	})

	t.Run("discard with no policy references", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repo := &Repository{r: r}

		err := repo.DiscardPolicy()
		assert.Nil(t, err)

		_, err = repo.r.GetReference(policy.PolicyIndexRef)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)
	})

	t.Run("discard after policy changes", func(t *testing.T) {
		repo := createTestRepositoryWithPolicy(t, "")

		initialRef, err := repo.r.GetReference(policy.PolicyRef)
		assert.Nil(t, err)

		if err := repo.r.SetReference(policy.PolicyIndexRef, gitinterface.ZeroHash); err != nil {
			t.Fatal(err)
		}

		err = repo.DiscardPolicy()
		assert.Nil(t, err)

		stagingRef, err := repo.r.GetReference(policy.PolicyIndexRef)
		assert.Nil(t, err)
		assert.Equal(t, initialRef, stagingRef)
	})
}

// TestStagePolicy_Selective verifies that selectively staging a subset of
// target names produces a PolicyStagingRef containing only those envelopes
// from PolicyIndexRef (with the rest taken from PolicyRef), and that
// PolicyIndexRef remains untouched.
func TestStagePolicy_Selective(t *testing.T) {
	repo := createTestRepositoryWithPolicy(t, "")

	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	require.Nil(t, err)
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	// Initialize protect-main as a real delegated policy file so we can
	// later mutate it and verify selective staging keeps the applied version.
	require.Nil(t, repo.InitializeTargets(testCtx, targetsSigner, "protect-main", false))
	require.Nil(t, repo.AddPrincipalToTargets(testCtx, targetsSigner, "protect-main", []tuf.Principal{gpgKey}, false))

	// Apply so PolicyRef contains the seeded delegations + protect-main.
	require.Nil(t, repo.StagePolicy(testCtx, "", []string{StageAllSentinel}, true, false))
	require.Nil(t, policy.Apply(testCtx, repo.r, false))

	// Capture the applied targets envelope blob to detect changes later.
	policyTip, err := repo.r.GetReference(policy.PolicyRef)
	require.Nil(t, err)
	policyTreeID, err := repo.r.GetCommitTreeID(policyTip)
	require.Nil(t, err)
	policyRootItems, err := repo.r.GetTreeItems(policyTreeID)
	require.Nil(t, err)
	policyMetadataItems, err := repo.r.GetTreeItems(policyRootItems["metadata"])
	require.Nil(t, err)
	appliedTargetsBlob := policyMetadataItems["targets.json"]
	appliedProtectMainBlob := policyMetadataItems["protect-main.json"]

	// Mutate the top-level targets envelope (adds a new rule referencing protect-release).
	require.Nil(t, repo.AddDelegation(testCtx, targetsSigner, policy.TargetsRoleName, "protect-release", []string{gpgKey.KeyID}, []string{"git:refs/heads/release"}, 1, false))
	// Also mutate the protect-main delegated policy file so the assertion
	// below has something to actually distinguish: PolicyRef's protect-main
	// vs. the divergent PolicyIndexRef version.
	require.Nil(t, repo.AddDelegation(testCtx, targetsSigner, "protect-main", "protect-main-sub-rule", []string{gpgKey.KeyID}, []string{"file:src/main/*"}, 1, false))

	// Selectively stage only the targets envelope.
	require.Nil(t, repo.StagePolicy(testCtx, "", []string{policy.TargetsRoleName}, true, false))

	// PolicyStagingRef should now have the new targets envelope but the
	// untouched protect-main envelope (no targets metadata file was created
	// for the new "protect-release" rule — it's a leaf rule).
	stagedTip, err := repo.r.GetReference(policy.PolicyStagingRef)
	require.Nil(t, err)
	stagedTreeID, err := repo.r.GetCommitTreeID(stagedTip)
	require.Nil(t, err)
	stagedRootItems, err := repo.r.GetTreeItems(stagedTreeID)
	require.Nil(t, err)
	stagedMetadataItems, err := repo.r.GetTreeItems(stagedRootItems["metadata"])
	require.Nil(t, err)

	assert.NotEqual(t, appliedTargetsBlob.String(), stagedMetadataItems["targets.json"].String(),
		"targets envelope should have been replaced by the PolicyIndexRef version in PolicyStagingRef")
	// protect-main wasn't modified in PolicyIndexRef, so it should match PolicyRef's version.
	assert.Equal(t, appliedProtectMainBlob.String(), stagedMetadataItems["protect-main.json"].String(),
		"protect-main envelope should be carried over from PolicyRef (not the PolicyIndexRef version)")
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

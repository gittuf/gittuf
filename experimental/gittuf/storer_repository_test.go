// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"errors"
	"testing"

	loadpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/loadpolicy"
	verifyopts "github.com/gittuf/gittuf/experimental/gittuf/options/verify"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorerRepositoryVerifyRef(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	_, rslPriv := applyAuthoredPolicyForTest(t, store)
	recordRefUpdateForTest(t, store, rslPriv)

	want, err := store.GetReference(authorTestRef)
	require.NoError(t, err)

	for name, opts := range map[string][]verifyopts.Option{
		"full":        nil,
		"latest only": {verifyopts.WithLatestOnly()},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tip, err := NewStorerRepository(store).VerifyRef(t.Context(), authorTestRef, opts...)
			require.NoError(t, err)
			assert.Equal(t, want, tip)
		})
	}
}

// The caller verifies a tip it may not have installed, so the reference's
// current value in the store is not consulted.
func TestStorerRepositoryVerifyRefReturnsTheAttestedTip(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	_, rslPriv := applyAuthoredPolicyForTest(t, store)
	recordRefUpdateForTest(t, store, rslPriv)

	attested, err := store.GetReference(authorTestRef)
	require.NoError(t, err)

	unrecorded := common.AddNTestCommitsToSpecifiedRef(t, store, authorTestRef, 1, opensshPEMForTest(t, rslPriv))
	require.False(t, unrecorded[0].Equal(attested.Bytes()))

	tip, err := NewStorerRepository(store).VerifyRef(t.Context(), authorTestRef, verifyopts.WithLatestOnly())
	require.NoError(t, err)
	assert.Equal(t, attested, tip)
}

func TestStorerRepositoryVerifyRefRejectsUnauthorizedSigner(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	applyAuthoredPolicyForTest(t, store)

	_, unauthorizedPriv := newTestED25519Key(t)
	recordRefUpdateForTest(t, store, unauthorizedPriv)

	_, err := NewStorerRepository(store).VerifyRef(t.Context(), authorTestRef)
	assert.Error(t, err)
}

func TestStorerRepositoryVerifyRefRequiresFullyQualifiedName(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	repo := NewStorerRepository(store)

	_, err := repo.VerifyRef(t.Context(), "main")
	assert.ErrorIs(t, err, ErrRefNotFullyQualified)

	_, err = repo.VerifyRef(t.Context(), authorTestRef, verifyopts.WithOverrideRefName("main"))
	assert.ErrorIs(t, err, ErrRefNotFullyQualified)

	_, err = repo.VerifyRefFromEntry(t.Context(), "main", nil)
	assert.ErrorIs(t, err, ErrRefNotFullyQualified)
}

func TestStorerRepositoryVerifyRefFromEntry(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	_, rslPriv := applyAuthoredPolicyForTest(t, store)
	recordRefUpdateForTest(t, store, rslPriv)

	entryID, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)
	want, err := store.GetReference(authorTestRef)
	require.NoError(t, err)

	tip, err := NewStorerRepository(store).VerifyRefFromEntry(t.Context(), authorTestRef, entryID)
	require.NoError(t, err)
	assert.Equal(t, want, tip)

	absent, err := githash.NewHash("0000000000000000000000000000000000000001")
	require.NoError(t, err)
	_, err = NewStorerRepository(store).VerifyRefFromEntry(t.Context(), authorTestRef, absent)
	assert.Error(t, err)
}

func TestStorerRepositoryLoadPolicy(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	params, _ := applyAuthoredPolicyForTest(t, store)

	loaded, err := NewStorerRepository(store).LoadPolicy(t.Context())
	require.NoError(t, err)

	rootID := principalIDForTest(t, params.RootPublicKeys[0])
	rslSignerID := principalIDForTest(t, params.RSLSignerPublicKeys[0])
	assert.Equal(t, RootInfo{Version: 1, RootKeyIDs: []string{rootID}}, loaded.Root())
	assert.ElementsMatch(t, []string{rootID, rslSignerID}, loaded.Principals())

	rule, err := loaded.Satisfies(authorTestRef, rslSignerID)
	require.NoError(t, err)
	assert.Equal(t, authorPolicyRuleName, rule)
}

func TestStorerRepositoryLoadPolicyWithoutPolicy(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)

	_, err := NewStorerRepository(store).LoadPolicy(t.Context())
	assert.Error(t, err)
}

func TestPolicyRootIsACopy(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	params, _ := applyAuthoredPolicyForTest(t, store)

	loaded, err := NewStorerRepository(store).LoadPolicy(t.Context())
	require.NoError(t, err)

	loaded.Root().RootKeyIDs[0] = "changed"
	assert.Equal(t, []string{principalIDForTest(t, params.RootPublicKeys[0])}, loaded.Root().RootKeyIDs)
}

func TestPolicySatisfies(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	params, _ := authorPolicyParamsForTest(t)
	secondPub, _ := newTestED25519Key(t)
	params.RSLSignerPublicKeys = append(params.RSLSignerPublicKeys, secondPub)
	params.RSLSignerThreshold = 2
	params.RefPatterns = []string{"git:refs/heads/*"}
	applyAuthoredPolicy(t, store, params)

	loaded, err := NewStorerRepository(store).LoadPolicy(t.Context())
	require.NoError(t, err)

	firstID := principalIDForTest(t, params.RSLSignerPublicKeys[0])
	secondID := principalIDForTest(t, secondPub)
	rootID := principalIDForTest(t, params.RootPublicKeys[0])

	tests := map[string]struct {
		refName      string
		principalIDs []string
		wantErr      error
	}{
		"threshold met":              {refName: authorTestRef, principalIDs: []string{firstID, secondID}},
		"one of two":                 {refName: authorTestRef, principalIDs: []string{firstID}, wantErr: ErrPolicyNotSatisfied},
		"principal outside the rule": {refName: authorTestRef, principalIDs: []string{firstID, rootID}, wantErr: ErrPolicyNotSatisfied},
		"unprotected ref":            {refName: "refs/tags/v1", principalIDs: []string{firstID, secondID}, wantErr: ErrRefNotProtected},
		"short ref name":             {refName: "main", principalIDs: []string{firstID, secondID}, wantErr: ErrRefNotFullyQualified},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rule, err := loaded.Satisfies(test.refName, test.principalIDs...)
			if test.wantErr != nil {
				assert.ErrorIs(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, authorPolicyRuleName, rule)
		})
	}
}

// applyTwoAuthoredPolicies applies a policy at version 1 and a successor at
// version 2, returning the RSL entry for the first.
func applyTwoAuthoredPolicies(t *testing.T, store *gitinterface.Repository) githash.Hash {
	t.Helper()

	params, _ := applyAuthoredPolicyForTest(t, store)

	first, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)

	successor := params
	successor.Version = 2
	applyAuthoredPolicy(t, store, successor)

	return first
}

func TestStorerRepositoryLoadPolicyWithEntry(t *testing.T) {
	t.Parallel()

	for name, opts := range map[string][]gitinterface.TestRepositoryOption{
		"SHA-1":   nil,
		"SHA-256": {gitinterface.WithSHA256Format()},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false, opts...)
			first := applyTwoAuthoredPolicies(t, store)
			repo := NewStorerRepository(store)

			loaded, err := repo.LoadPolicy(t.Context(), loadpolicyopts.WithEntry(first))
			require.NoError(t, err)
			assert.Equal(t, uint64(1), loaded.Root().Version)

			loaded, err = repo.LoadPolicy(t.Context())
			require.NoError(t, err)
			assert.Equal(t, uint64(2), loaded.Root().Version, "without WithEntry the latest policy entry is read")
		})
	}
}

func TestStorerRepositoryLoadPolicyRejectsInvalidEntries(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	_, rslPriv := applyAuthoredPolicyForTest(t, store)
	keyPEM := opensshPEMForTest(t, rslPriv)

	policyEntry, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)

	require.NoError(t, rsl.NewAnnotationEntry([]githash.Hash{policyEntry}, false, "note").CommitUsingSpecificKey(store, keyPEM))
	annotation, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)

	recordRefUpdateForTest(t, store, rslPriv)
	otherRefEntry, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)

	tests := map[string]struct {
		entryID githash.Hash
		errMsg  string
	}{
		"zero entry":          {entryID: githash.Hash{}, errMsg: "must not be zero"},
		"annotation entry":    {entryID: annotation, errMsg: "not a reference entry"},
		"entry for other ref": {entryID: otherRefEntry, errMsg: "not the policy ref"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := NewStorerRepository(store).LoadPolicy(t.Context(), loadpolicyopts.WithEntry(test.entryID))
			assert.ErrorContains(t, err, test.errMsg)
		})
	}
}

// GetEntry parses any commit that reads as an entry. The RSL it belongs to
// decides whether it is one.
func TestStorerRepositoryLoadPolicyRejectsEntryOutsideTheRSL(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	params, rslPriv := authorPolicyParamsForTest(t)
	keyPEM := opensshPEMForTest(t, rslPriv)

	recordRefUpdateForTest(t, store, rslPriv)
	rslTip, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)

	tree, err := AuthorPolicy(t.Context(), store, params)
	require.NoError(t, err)
	policyTip, err := store.Commit(tree, policy.PolicyRef, "Unapplied policy", false)
	require.NoError(t, err)
	require.NoError(t, rsl.NewReferenceEntry(policy.PolicyRef, policyTip).CommitUsingSpecificKey(store, keyPEM))
	detached, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)
	require.NoError(t, store.SetReference(rsl.Ref, rslTip))

	_, err = NewStorerRepository(store).LoadPolicy(t.Context(), loadpolicyopts.WithEntry(detached))
	assert.ErrorContains(t, err, "not reachable")
}

type failingKnowsCommitStore struct {
	gitstore.Storer
}

var errKnowsCommit = errors.New("ancestry check failed")

func (failingKnowsCommitStore) KnowsCommit(githash.Hash, githash.Hash) (bool, error) {
	return false, errKnowsCommit
}

func TestStorerRepositoryLoadPolicySurfacesMembershipCheckFailure(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	applyAuthoredPolicyForTest(t, store)
	entryID, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)

	_, err = NewStorerRepository(failingKnowsCommitStore{Storer: store}).LoadPolicy(t.Context(), loadpolicyopts.WithEntry(entryID))
	assert.ErrorIs(t, err, errKnowsCommit)
}

func TestStorerRepositoryLoadPolicyWithInitialRootKeys(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	params, _ := applyAuthoredPolicyForTest(t, store)
	repo := NewStorerRepository(store)

	loaded, err := repo.LoadPolicy(t.Context(), loadpolicyopts.WithInitialRootKeys(params.RootPublicKeys...))
	require.NoError(t, err)
	assert.Equal(t, uint64(1), loaded.Root().Version)

	otherPub, _ := newTestED25519Key(t)
	_, err = repo.LoadPolicy(t.Context(), loadpolicyopts.WithInitialRootKeys(otherPub))
	assert.Error(t, err, "a history whose first root is not the declared anchor must be refused")

	_, err = repo.LoadPolicy(t.Context(), loadpolicyopts.WithInitialRootKeys("not a key"))
	assert.ErrorContains(t, err, "unable to create ssh public key")
}

// rslSwapStore returns a different RSL tip after the first read, as a reader
// sees a writer rewriting the RSL mid-load.
type rslSwapStore struct {
	gitstore.Storer
	swapTo githash.Hash
	reads  *int
}

func (s rslSwapStore) GetReference(refName string) (githash.Hash, error) {
	if refName == rsl.Ref {
		*s.reads++
		if *s.reads > 1 {
			return s.swapTo, nil
		}
	}

	return s.Storer.GetReference(refName)
}

// A writer rewriting the RSL mid-load must not change the history the
// load reads. Without the snapshot, the membership check would see one
// RSL and the policy load another with no applied policy to verify against.
func TestStorerRepositoryLoadPolicyWhenTheRSLIsRewritten(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		unsigned bool
		anchored bool
		wantErr  bool
	}{
		"signed metadata":               {},
		"unsigned metadata":             {unsigned: true, wantErr: true},
		"metadata signed by other keys": {anchored: true, wantErr: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
			params, rslPriv := authorPolicyParamsForTest(t)
			keyPEM := opensshPEMForTest(t, rslPriv)

			recordRefUpdateForTest(t, store, rslPriv)
			policyFreeRSL, err := store.GetReference(rsl.Ref)
			require.NoError(t, err)

			tree, err := AuthorPolicy(t.Context(), store, params)
			require.NoError(t, err)
			if test.unsigned {
				tree = stripSignaturesForTest(t, store, tree)
			}
			policyTip, err := store.Commit(tree, policy.PolicyRef, "Policy", false)
			require.NoError(t, err)
			require.NoError(t, rsl.NewReferenceEntry(policy.PolicyRef, policyTip).CommitUsingSpecificKey(store, keyPEM))
			entryID, err := store.GetReference(rsl.Ref)
			require.NoError(t, err)

			opts := []loadpolicyopts.Option{loadpolicyopts.WithEntry(entryID)}
			if test.anchored {
				anchor, _ := newTestED25519Key(t)
				opts = append(opts, loadpolicyopts.WithInitialRootKeys(anchor))
			}

			reads := 0
			swapped := rslSwapStore{Storer: store, swapTo: policyFreeRSL, reads: &reads}

			_, err = NewStorerRepository(swapped).LoadPolicy(t.Context(), opts...)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, 1, reads, "the RSL tip must be read once and reused")
		})
	}
}

// stripSignaturesForTest rewrites the policy tree at treeID with every
// envelope's signatures removed.
func stripSignaturesForTest(t *testing.T, store gitstore.Storer, treeID githash.Hash) githash.Hash {
	t.Helper()

	commitID, err := store.Commit(treeID, "refs/gittuf/strip-signatures", "Signed policy", false)
	require.NoError(t, err)

	state, err := policy.LoadStateFromCommit(store, commitID)
	require.NoError(t, err)

	state.Metadata.RootEnvelope.Signatures = nil
	state.Metadata.TargetsEnvelope.Signatures = nil

	unsigned, err := state.WriteTree(store)
	require.NoError(t, err)

	return unsigned
}

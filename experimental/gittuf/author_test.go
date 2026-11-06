// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

const (
	authorTestRefPattern = "git:refs/*"
	authorTestRef        = "refs/heads/main"
)

func newTestED25519Key(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	return pub, priv
}

// opensshPEMForTest serializes priv into the OpenSSH PEM bytes that
// CommitUsingSpecificKey accepts.
func opensshPEMForTest(t *testing.T, priv ed25519.PrivateKey) []byte {
	t.Helper()

	block, err := gossh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)

	return pem.EncodeToMemory(block)
}

func principalIDForTest(t *testing.T, pub crypto.PublicKey) string {
	t.Helper()

	principal, err := newPrincipal(pub)
	require.NoError(t, err)

	return principal.ID()
}

func authorPolicyParamsForTest(t *testing.T) (AuthorPolicyParams, ed25519.PrivateKey) {
	t.Helper()

	rootPub, rootPriv := newTestED25519Key(t)
	rslPub, rslPriv := newTestED25519Key(t)

	return AuthorPolicyParams{
		RootSigners:         []crypto.Signer{rootPriv},
		RootPublicKeys:      []crypto.PublicKey{rootPub},
		RootThreshold:       1,
		RSLSignerPublicKeys: []crypto.PublicKey{rslPub},
		RSLSignerThreshold:  1,
		RefPatterns:         []string{authorTestRefPattern},
		Expires:             time.Now().Add(365 * 24 * time.Hour),
	}, rslPriv
}

// applyAuthoredPolicy stages and applies the policy authored from p, the way
// a caller of AuthorPolicy installs it.
func applyAuthoredPolicy(t *testing.T, store *gitinterface.Repository, p AuthorPolicyParams) {
	t.Helper()

	tree, err := AuthorPolicy(t.Context(), store, p)
	require.NoError(t, err)
	applyPolicyTreeForTest(t, store, tree)
}

// applyPolicyTreeForTest stages tree and applies it.
func applyPolicyTreeForTest(t *testing.T, store *gitinterface.Repository, tree githash.Hash) {
	t.Helper()

	tip, err := store.Commit(tree, policy.PolicyStagingRef, "Stage authored policy", false)
	require.NoError(t, err)
	require.NoError(t, rsl.NewReferenceEntry(policy.PolicyStagingRef, tip).Commit(store, false))
	require.NoError(t, policy.Apply(t.Context(), store, false))
}

func applyAuthoredPolicyForTest(t *testing.T, store *gitinterface.Repository) (AuthorPolicyParams, ed25519.PrivateKey) {
	t.Helper()

	params, rslPriv := authorPolicyParamsForTest(t)
	applyAuthoredPolicy(t, store, params)

	return params, rslPriv
}

func recordRefUpdateForTest(t *testing.T, store *gitinterface.Repository, signingKey ed25519.PrivateKey) {
	t.Helper()

	keyPEM := opensshPEMForTest(t, signingKey)
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, store, authorTestRef, 1, keyPEM)
	require.NoError(t, rsl.NewReferenceEntry(authorTestRef, commitIDs[0]).CommitUsingSpecificKey(store, keyPEM))
}

func TestAuthorPolicyWritesNoReferences(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	params, _ := authorPolicyParamsForTest(t)

	tree, err := AuthorPolicy(t.Context(), store, params)
	require.NoError(t, err)
	assert.False(t, tree.IsZero())

	for _, ref := range []string{policy.PolicyRef, policy.PolicyStagingRef, rsl.Ref} {
		_, err := store.GetReference(ref)
		assert.ErrorIs(t, err, gitstore.ErrReferenceNotFound, "AuthorPolicy must not write %s", ref)
	}
}

func TestAuthorPolicyIsAppliedByTheEngine(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	_, rslPriv := applyAuthoredPolicyForTest(t, store)

	recordRefUpdateForTest(t, store, rslPriv)

	tip, err := NewStorerRepository(store).VerifyRef(t.Context(), authorTestRef)
	require.NoError(t, err)
	assert.False(t, tip.IsZero())
}

func TestAuthorPolicyStampsVersion(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		requested uint64
		want      uint64
	}{
		"unset defaults to one": {requested: 0, want: 1},
		"explicit one":          {requested: 1, want: 1},
		"later version":         {requested: 4, want: 4},
		"large version":         {requested: 1 << 40, want: 1 << 40},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
			params, _ := authorPolicyParamsForTest(t)
			params.Version = test.requested
			applyAuthoredPolicy(t, store, params)

			loaded, err := NewStorerRepository(store).LoadPolicy(t.Context())
			require.NoError(t, err)
			assert.Equal(t, test.want, loaded.Root().Version)
		})
	}
}

func TestAuthorPolicySuccessorIsAcceptedByTheEngine(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	params, _ := applyAuthoredPolicyForTest(t, store)

	successor := params
	successor.Version = 2
	applyAuthoredPolicy(t, store, successor)

	loaded, err := NewStorerRepository(store).LoadPolicy(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(2), loaded.Root().Version)
}

func TestAuthorPolicySuccessorWithUnsanctionedRootIsRefused(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	params, _ := applyAuthoredPolicyForTest(t, store)

	successor := params
	successor.Version = 2
	otherPub, otherPriv := newTestED25519Key(t)
	successor.RootSigners = []crypto.Signer{otherPriv}
	successor.RootPublicKeys = []crypto.PublicKey{otherPub}

	tree, err := AuthorPolicy(t.Context(), store, successor)
	require.NoError(t, err)
	tip, err := store.Commit(tree, policy.PolicyStagingRef, "Stage unsanctioned root", false)
	require.NoError(t, err)
	require.NoError(t, rsl.NewReferenceEntry(policy.PolicyStagingRef, tip).Commit(store, false))

	assert.Error(t, policy.Apply(t.Context(), store, false), "a root the applied keys never signed must not be applied")
}

func TestAuthorPolicyWithMultipleRootSigners(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	params, rslPriv := authorPolicyParamsForTest(t)

	secondPub, secondPriv := newTestED25519Key(t)
	params.RootPublicKeys = append(params.RootPublicKeys, secondPub)
	params.RootSigners = append(params.RootSigners, secondPriv)
	params.RootThreshold = 2
	applyAuthoredPolicy(t, store, params)

	loaded, err := NewStorerRepository(store).LoadPolicy(t.Context())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{principalIDForTest(t, params.RootPublicKeys[0]), principalIDForTest(t, secondPub)}, loaded.Root().RootKeyIDs)

	recordRefUpdateForTest(t, store, rslPriv)
	_, err = NewStorerRepository(store).VerifyRef(t.Context(), authorTestRef)
	assert.NoError(t, err, "metadata signed to its threshold must verify")
}

func TestAuthorPolicyHonoursRSLSignerThreshold(t *testing.T) {
	t.Parallel()

	// Two authorized RSL signers, one of which signs the update. Only the
	// threshold differs between the cases.
	tests := map[string]struct {
		threshold int
		verifies  bool
	}{
		"unset defaults to one": {threshold: 0, verifies: true},
		"threshold one":         {threshold: 1, verifies: true},
		"threshold two":         {threshold: 2, verifies: false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
			params, rslPriv := authorPolicyParamsForTest(t)
			secondPub, _ := newTestED25519Key(t)
			params.RSLSignerPublicKeys = append(params.RSLSignerPublicKeys, secondPub)
			params.RSLSignerThreshold = test.threshold
			applyAuthoredPolicy(t, store, params)

			recordRefUpdateForTest(t, store, rslPriv)

			_, err := NewStorerRepository(store).VerifyRef(t.Context(), authorTestRef)
			if test.verifies {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAuthorPolicyDeduplicatesRepeatedKeys(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	params, rslPriv := authorPolicyParamsForTest(t)

	rootPub := params.RootPublicKeys[0]
	rootPriv := params.RootSigners[0]
	rslPub := params.RSLSignerPublicKeys[0]
	params.RootPublicKeys = []crypto.PublicKey{rootPub, rootPub}
	params.RootSigners = []crypto.Signer{rootPriv, rootPriv}
	params.RSLSignerPublicKeys = []crypto.PublicKey{rslPub, rslPub}
	applyAuthoredPolicy(t, store, params)

	loaded, err := NewStorerRepository(store).LoadPolicy(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{principalIDForTest(t, rootPub)}, loaded.Root().RootKeyIDs)
	assert.ElementsMatch(t, []string{principalIDForTest(t, rootPub), principalIDForTest(t, rslPub)}, loaded.Principals())

	recordRefUpdateForTest(t, store, rslPriv)
	_, err = NewStorerRepository(store).VerifyRef(t.Context(), authorTestRef)
	assert.NoError(t, err)
}

// unsupportedSigner is a crypto.Signer holding a key type SSH cannot represent.
type unsupportedSigner struct{}

func (unsupportedSigner) Public() crypto.PublicKey { return struct{}{} }

func (unsupportedSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return nil, errors.New("unsupported")
}

func TestAuthorPolicyValidation(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		mutate func(*AuthorPolicyParams)
		errMsg string
	}{
		"no root signers": {
			mutate: func(p *AuthorPolicyParams) { p.RootSigners = nil },
			errMsg: "at least one root signer must be provided",
		},
		"nil root signer": {
			mutate: func(p *AuthorPolicyParams) { p.RootSigners = []crypto.Signer{nil} },
			errMsg: "root signer must not be nil",
		},
		"unsupported root signer": {
			mutate: func(p *AuthorPolicyParams) { p.RootSigners = []crypto.Signer{unsupportedSigner{}} },
			errMsg: "failed to create ssh signer",
		},
		"no root public keys": {
			mutate: func(p *AuthorPolicyParams) { p.RootPublicKeys = nil },
			errMsg: "at least one root public key",
		},
		"unsupported root public key": {
			mutate: func(p *AuthorPolicyParams) { p.RootPublicKeys = []crypto.PublicKey{"not a key"} },
			errMsg: "unable to create ssh public key",
		},
		"root threshold below one": {
			mutate: func(p *AuthorPolicyParams) { p.RootThreshold = 0 },
			errMsg: "root threshold must be at least 1",
		},
		"root threshold above key count": {
			mutate: func(p *AuthorPolicyParams) { p.RootThreshold = 2 },
			errMsg: "root threshold 2 exceeds the 1 distinct root public keys",
		},
		"root threshold above signer count": {
			mutate: func(p *AuthorPolicyParams) {
				otherPub, _ := newTestED25519Key(t)
				p.RootPublicKeys = append(p.RootPublicKeys, otherPub)
				p.RootThreshold = 2
			},
			errMsg: "root threshold 2 exceeds the 1 distinct root signers",
		},
		"duplicate root signer counted once": {
			mutate: func(p *AuthorPolicyParams) {
				otherPub, _ := newTestED25519Key(t)
				p.RootSigners = []crypto.Signer{p.RootSigners[0], p.RootSigners[0]}
				p.RootPublicKeys = append(p.RootPublicKeys, otherPub)
				p.RootThreshold = 2
			},
			errMsg: "root threshold 2 exceeds the 1 distinct root signers",
		},
		"root signer not among root public keys": {
			mutate: func(p *AuthorPolicyParams) {
				otherPub, _ := newTestED25519Key(t)
				p.RootPublicKeys = []crypto.PublicKey{otherPub}
			},
			errMsg: "is not among the root public keys",
		},
		"no RSL signer public keys": {
			mutate: func(p *AuthorPolicyParams) { p.RSLSignerPublicKeys = nil },
			errMsg: "at least one RSL signer public key",
		},
		"unsupported RSL signer public key": {
			mutate: func(p *AuthorPolicyParams) { p.RSLSignerPublicKeys = []crypto.PublicKey{"not a key"} },
			errMsg: "unable to create ssh public key",
		},
		"negative RSL signer threshold": {
			mutate: func(p *AuthorPolicyParams) { p.RSLSignerThreshold = -1 },
			errMsg: "RSL signer threshold must not be negative",
		},
		"RSL threshold above key count": {
			mutate: func(p *AuthorPolicyParams) { p.RSLSignerThreshold = 3 },
			errMsg: "RSL signer threshold 3 exceeds",
		},
		"duplicate RSL signer key counted once": {
			mutate: func(p *AuthorPolicyParams) {
				p.RSLSignerPublicKeys = []crypto.PublicKey{p.RSLSignerPublicKeys[0], p.RSLSignerPublicKeys[0]}
				p.RSLSignerThreshold = 2
			},
			errMsg: "RSL signer threshold 2 exceeds the 1 distinct RSL signer public keys",
		},
		"no ref patterns": {
			mutate: func(p *AuthorPolicyParams) { p.RefPatterns = nil },
			errMsg: "at least one ref pattern",
		},
		"zero expiry": {
			mutate: func(p *AuthorPolicyParams) { p.Expires = time.Time{} },
			errMsg: "expiry must be provided",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
			params, _ := authorPolicyParamsForTest(t)
			test.mutate(&params)

			_, err := AuthorPolicy(t.Context(), store, params)
			require.ErrorContains(t, err, test.errMsg)

			for _, ref := range []string{policy.PolicyRef, policy.PolicyStagingRef} {
				_, err := store.GetReference(ref)
				assert.ErrorIs(t, err, gitstore.ErrReferenceNotFound, "%s must not be written", ref)
			}
		})
	}
}

type failingTreeStore struct {
	gitstore.Storer
}

var errTreeWrite = errors.New("tree write failed")

func (failingTreeStore) WriteTree([]gitstore.TreeEntry) (githash.Hash, error) {
	return nil, errTreeWrite
}

func TestAuthorPolicyPropagatesTreeWriteError(t *testing.T) {
	t.Parallel()

	params, _ := authorPolicyParamsForTest(t)
	store := failingTreeStore{Storer: gitinterface.CreateTestGitRepository(t, t.TempDir(), false)}

	_, err := AuthorPolicy(context.Background(), store, params)
	assert.ErrorIs(t, err, errTreeWrite)
}

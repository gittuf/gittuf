// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

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
	ipolicy "github.com/gittuf/gittuf/internal/policy"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	testRefPattern = "git:refs/*"
	testRef        = "refs/heads/main"
)

func newTestED25519(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	return pub, priv
}

// opensshPEM serializes a raw ed25519 private key into the OpenSSH PEM bytes
// that gitinterface.CommitUsingSpecificKey understands.
func opensshPEM(t *testing.T, priv ed25519.PrivateKey) []byte {
	t.Helper()

	block, err := ssh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)

	return pem.EncodeToMemory(block)
}

func newTestStore(t *testing.T) *gitinterface.Repository {
	t.Helper()

	return gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
}

func testParams(t *testing.T) (BootstrapParams, ed25519.PrivateKey) {
	t.Helper()

	rootPub, rootPriv := newTestED25519(t)
	rslPub, rslPriv := newTestED25519(t)

	return BootstrapParams{
		RootSigners:         []crypto.Signer{rootPriv},
		RootPublicKeys:      []crypto.PublicKey{rootPub},
		RootThreshold:       1,
		RSLSignerPublicKeys: []crypto.PublicKey{rslPub},
		RSLSignerThreshold:  1,
		RefPatterns:         []string{testRefPattern},
		Expires:             time.Now().Add(365 * 24 * time.Hour),
	}, rslPriv
}

func bootstrapForTest(t *testing.T, store *gitinterface.Repository) (BootstrapParams, ed25519.PrivateKey) {
	t.Helper()

	params, rslPriv := testParams(t)
	_, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)

	return params, rslPriv
}

func recordRefUpdate(t *testing.T, store *gitinterface.Repository, signingKey ed25519.PrivateKey) {
	t.Helper()

	keyPEM := opensshPEM(t, signingKey)
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, store, testRef, 1, keyPEM)
	require.NoError(t, rsl.NewReferenceEntry(testRef, commitIDs[0]).CommitUsingSpecificKey(store, keyPEM))
}

func TestPrincipalID(t *testing.T) {
	t.Parallel()

	pub, priv := newTestED25519(t)

	id, err := PrincipalID(pub)
	require.NoError(t, err)

	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	assert.Equal(t, ssh.FingerprintSHA256(sshPub), id)

	sshSigner, err := ssh.NewSignerFromSigner(priv)
	require.NoError(t, err)
	assert.Equal(t, ssh.FingerprintSHA256(sshSigner.PublicKey()), id, "principal ID must match the signing key's ID")
}

func TestBootstrapAndVerifyRef(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	_, rslPriv := bootstrapForTest(t, store)

	recordRefUpdate(t, store, rslPriv)

	tip, err := VerifyRef(context.Background(), store, testRef)
	require.NoError(t, err)
	assert.False(t, tip.IsZero())
}

func TestVerifyRefRejectsUnauthorizedSigner(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	bootstrapForTest(t, store)

	_, unauthorizedPriv := newTestED25519(t)
	recordRefUpdate(t, store, unauthorizedPriv)

	_, err := VerifyRef(context.Background(), store, testRef)
	assert.Error(t, err)
}

func TestVerifyRefFromEntry(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	_, rslPriv := bootstrapForTest(t, store)

	recordRefUpdate(t, store, rslPriv)

	entryID, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)

	tip, err := VerifyRefFromEntry(context.Background(), store, testRef, entryID)
	require.NoError(t, err)
	assert.False(t, tip.IsZero())
}

func TestInspectRoot(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, _ := bootstrapForTest(t, store)

	rslSignerID, err := PrincipalID(params.RSLSignerPublicKeys[0])
	require.NoError(t, err)
	rootKeyID, err := PrincipalID(params.RootPublicKeys[0])
	require.NoError(t, err)

	info, err := InspectRoot(context.Background(), store)
	require.NoError(t, err)

	assert.Equal(t, uint64(1), info.Version)
	assert.Equal(t, []string{rootKeyID}, info.RootKeyIDs)
	assert.Contains(t, info.AuthorizedRSLSigners, rslSignerID)
}

func TestBootstrapStampsRootVersion(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		requested uint64
		want      uint64
	}{
		"unset defaults to one": {requested: 0, want: 1},
		"explicit one":          {requested: 1, want: 1},
		"later profile":         {requested: 4, want: 4},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			params, _ := testParams(t)
			params.RootVersion = test.requested

			_, err := Bootstrap(context.Background(), store, params)
			require.NoError(t, err)

			info, err := InspectRoot(context.Background(), store)
			require.NoError(t, err)
			assert.Equal(t, test.want, info.Version)
		})
	}
}

func TestBootstrapRejectsReinitialization(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	bootstrapForTest(t, store)

	params, _ := testParams(t)
	_, err := Bootstrap(context.Background(), store, params)
	assert.ErrorContains(t, err, "policy already initialized")
}

func TestBootstrapValidation(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		mutate func(*BootstrapParams)
		errMsg string
	}{
		"no root signers": {
			mutate: func(p *BootstrapParams) { p.RootSigners = nil },
			errMsg: "at least one root signer must be provided",
		},
		"no root public keys": {
			mutate: func(p *BootstrapParams) { p.RootPublicKeys = nil },
			errMsg: "at least one root public key",
		},
		"root threshold below one": {
			mutate: func(p *BootstrapParams) { p.RootThreshold = 0 },
			errMsg: "root threshold must be at least 1",
		},
		"root threshold above key count": {
			mutate: func(p *BootstrapParams) { p.RootThreshold = 2 },
			errMsg: "root threshold 2 exceeds the 1 distinct root public keys",
		},
		"root threshold above signer count": {
			mutate: func(p *BootstrapParams) {
				otherPub, _ := newTestED25519(t)
				p.RootPublicKeys = append(p.RootPublicKeys, otherPub)
				p.RootThreshold = 2
			},
			errMsg: "root threshold 2 exceeds the 1 distinct root signers",
		},
		"RSL threshold above key count": {
			mutate: func(p *BootstrapParams) { p.RSLSignerThreshold = 3 },
			errMsg: "RSL signer threshold 3 exceeds",
		},
		"no RSL signer public keys": {
			mutate: func(p *BootstrapParams) { p.RSLSignerPublicKeys = nil },
			errMsg: "at least one RSL signer public key",
		},
		"negative RSL signer threshold": {
			mutate: func(p *BootstrapParams) { p.RSLSignerThreshold = -1 },
			errMsg: "RSL signer threshold must not be negative",
		},
		"nil root signer": {
			mutate: func(p *BootstrapParams) { p.RootSigners = []crypto.Signer{nil} },
			errMsg: "root signer must not be nil",
		},
		"unsupported root public key": {
			mutate: func(p *BootstrapParams) { p.RootPublicKeys = []crypto.PublicKey{"not a key"} },
			errMsg: "unable to create ssh public key",
		},
		"unsupported RSL signer public key": {
			mutate: func(p *BootstrapParams) { p.RSLSignerPublicKeys = []crypto.PublicKey{"not a key"} },
			errMsg: "unable to create ssh public key",
		},
		"no ref patterns": {
			mutate: func(p *BootstrapParams) { p.RefPatterns = nil },
			errMsg: "at least one ref pattern",
		},
		"zero expiry": {
			mutate: func(p *BootstrapParams) { p.Expires = time.Time{} },
			errMsg: "expiry must be provided",
		},
		"root signer not among root public keys": {
			mutate: func(p *BootstrapParams) {
				otherPub, _ := newTestED25519(t)
				p.RootPublicKeys = []crypto.PublicKey{otherPub}
			},
			errMsg: "is not among the root public keys",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			params, _ := testParams(t)
			test.mutate(&params)

			_, err := Bootstrap(context.Background(), newTestStore(t), params)
			assert.ErrorContains(t, err, test.errMsg)
		})
	}
}

func TestBootstrapHonoursRSLSignerThreshold(t *testing.T) {
	t.Parallel()

	// Two authorized RSL signers, one of which signs the ref update. Only the
	// threshold differs between the two cases.
	tests := map[string]struct {
		threshold int
		verifies  bool
	}{
		"one of two signs, threshold one": {threshold: 1, verifies: true},
		"one of two signs, threshold two": {threshold: 2, verifies: false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			params, rslPriv := testParams(t)

			secondPub, _ := newTestED25519(t)
			params.RSLSignerPublicKeys = append(params.RSLSignerPublicKeys, secondPub)
			params.RSLSignerThreshold = test.threshold

			_, err := Bootstrap(context.Background(), store, params)
			require.NoError(t, err)

			recordRefUpdate(t, store, rslPriv)

			_, err = VerifyRef(context.Background(), store, testRef)
			if test.verifies {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAuthorCommitsOntoParent(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, _ := testParams(t)

	oldTip, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)

	successor := params
	successor.RootVersion = 2

	tree, err := Author(context.Background(), store, successor)
	require.NoError(t, err)

	newTip, err := store.Commit(tree, Ref, "Transition gittuf policy", false)
	require.NoError(t, err)

	parents, err := store.GetCommitParentIDs(newTip)
	require.NoError(t, err)
	require.Len(t, parents, 1)
	assert.Equal(t, oldTip.String(), parents[0].String(), "authored successor must descend from the applied tip")

	require.NoError(t, VerifyTransition(context.Background(), store, oldTip, newTip))
}

func TestVerifyTransition(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		appliedVersion   uint64
		successorVersion uint64
		rotateRootKey    bool
		wantErr          error
	}{
		"same version":      {appliedVersion: 1, successorVersion: 1},
		"version advances":  {appliedVersion: 1, successorVersion: 4},
		"version regresses": {appliedVersion: 3, successorVersion: 2, wantErr: ErrMetadataRollbackDetected},
		"unsanctioned root": {appliedVersion: 1, successorVersion: 2, rotateRootKey: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			params, _ := testParams(t)
			params.RootVersion = test.appliedVersion

			oldTip, err := Bootstrap(context.Background(), store, params)
			require.NoError(t, err)

			successor := params
			successor.RootVersion = test.successorVersion
			if test.rotateRootKey {
				otherPub, otherPriv := newTestED25519(t)
				successor.RootSigners = []crypto.Signer{otherPriv}
				successor.RootPublicKeys = []crypto.PublicKey{otherPub}
			}

			tree, err := Author(context.Background(), store, successor)
			require.NoError(t, err)

			newTip, err := store.Commit(tree, Ref, "Transition gittuf policy", false)
			require.NoError(t, err)

			err = VerifyTransition(context.Background(), store, oldTip, newTip)
			switch {
			case test.wantErr != nil:
				assert.ErrorIs(t, err, test.wantErr)
			case test.rotateRootKey:
				assert.Error(t, err, "a root the applied key set never sanctioned must not verify")
			default:
				assert.NoError(t, err)
			}
		})
	}
}

func TestBootstrapWithMultipleRootSigners(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, rslPriv := testParams(t)

	secondPub, secondPriv := newTestED25519(t)
	params.RootPublicKeys = append(params.RootPublicKeys, secondPub)
	params.RootSigners = append(params.RootSigners, secondPriv)
	params.RootThreshold = 2

	_, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)

	firstID, err := PrincipalID(params.RootPublicKeys[0])
	require.NoError(t, err)
	secondID, err := PrincipalID(secondPub)
	require.NoError(t, err)

	info, err := InspectRoot(context.Background(), store)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{firstID, secondID}, info.RootKeyIDs)

	recordRefUpdate(t, store, rslPriv)
	_, err = VerifyRef(context.Background(), store, testRef)
	assert.NoError(t, err, "metadata signed to its threshold must verify")
}

func TestBootstrapRejectsUnsatisfiableThresholdBeforeWriting(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, _ := testParams(t)

	secondPub, _ := newTestED25519(t)
	params.RootPublicKeys = append(params.RootPublicKeys, secondPub)
	params.RootThreshold = 2

	_, err := Bootstrap(context.Background(), store, params)
	require.ErrorContains(t, err, "root threshold 2 exceeds the 1 distinct root signers")

	for _, ref := range []string{Ref, StagingRef} {
		_, err := store.GetReference(ref)
		assert.ErrorIs(t, err, gitstore.ErrReferenceNotFound, "%s must not be written when authoring is rejected", ref)
	}
}

func TestVerifyTransitionRequiresDescendant(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, _ := testParams(t)

	oldTip, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)

	// An unrelated policy commit, authored from the same keys at the same
	// version, so only the missing ancestry distinguishes it.
	tree, err := Author(context.Background(), store, params)
	require.NoError(t, err)
	unrelatedTip, err := store.Commit(tree, "refs/gittuf/policy-unrelated", "Unrelated policy", false)
	require.NoError(t, err)

	parents, err := store.GetCommitParentIDs(unrelatedTip)
	require.NoError(t, err)
	require.Empty(t, parents, "the unrelated commit must not descend from the applied tip")

	err = VerifyTransition(context.Background(), store, oldTip, unrelatedTip)
	assert.ErrorIs(t, err, ErrNotAncestor)
}

func TestBootstrapStampsLargeRootVersion(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, _ := testParams(t)
	params.RootVersion = 1 << 40

	_, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)

	info, err := InspectRoot(context.Background(), store)
	require.NoError(t, err)
	assert.Equal(t, uint64(1<<40), info.Version)
}

func TestProtectsGitNamespace(t *testing.T) {
	t.Parallel()

	pub, _ := newTestED25519(t)
	principal, err := newPrincipal(pub)
	require.NoError(t, err)

	metadata := tufv02.NewTargetsMetadata()
	require.NoError(t, metadata.AddPrincipal(principal))

	rules := map[string][]string{
		"git-only":  {"git:refs/heads/*"},
		"file-only": {"file:*"},
		"mixed":     {"file:*", "git:refs/tags/*"},
	}
	for name, patterns := range rules {
		require.NoError(t, metadata.AddRule(name, []string{principal.ID()}, patterns, 1))
	}

	want := map[string]bool{"git-only": true, "file-only": false, "mixed": true}
	for _, rule := range metadata.GetRules() {
		expected, known := want[rule.ID()]
		if !known {
			continue
		}
		assert.Equal(t, expected, protectsGitNamespace(rule), "rule %s", rule.ID())
	}
}

func TestInspectRootExcludesFileOnlyRules(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, _ := testParams(t)
	params.RefPatterns = []string{"file:*"}

	_, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)

	info, err := InspectRoot(context.Background(), store)
	require.NoError(t, err)
	assert.Empty(t, info.AuthorizedRSLSigners, "a rule protecting only file paths does not authorize RSL signatures")
	assert.NotEmpty(t, info.RootKeyIDs)
}

func TestAuthorWritesNoReferences(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, _ := testParams(t)

	tree, err := Author(context.Background(), store, params)
	require.NoError(t, err)
	assert.False(t, tree.IsZero())

	for _, ref := range []string{Ref, StagingRef, rsl.Ref} {
		_, err := store.GetReference(ref)
		assert.ErrorIs(t, err, gitstore.ErrReferenceNotFound, "Author must not write %s", ref)
	}
}

func TestAuthorRejectsInvalidParams(t *testing.T) {
	t.Parallel()

	params, _ := testParams(t)
	params.RefPatterns = nil

	_, err := Author(context.Background(), newTestStore(t), params)
	assert.ErrorContains(t, err, "at least one ref pattern")
}

func TestInspectRootWithoutPolicy(t *testing.T) {
	t.Parallel()

	_, err := InspectRoot(context.Background(), newTestStore(t))
	assert.Error(t, err)
}

func TestVerifyTransitionWithUnknownCommit(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, _ := testParams(t)

	oldTip, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)

	absent, err := githash.NewHash("0000000000000000000000000000000000000001")
	require.NoError(t, err)

	assert.Error(t, VerifyTransition(context.Background(), store, oldTip, absent))
	assert.Error(t, VerifyTransition(context.Background(), store, absent, oldTip))
}

func TestPrincipalIDRejectsUnsupportedKey(t *testing.T) {
	t.Parallel()

	_, err := PrincipalID("not a key")
	assert.Error(t, err)
}

func TestVerifyRefFromEntryRejectsUnknownEntry(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	_, rslPriv := bootstrapForTest(t, store)
	recordRefUpdate(t, store, rslPriv)

	absent, err := githash.NewHash("0000000000000000000000000000000000000001")
	require.NoError(t, err)

	_, err = VerifyRefFromEntry(context.Background(), store, testRef, absent)
	assert.Error(t, err)
}

func TestBootstrapCountsThresholdsAgainstDistinctKeys(t *testing.T) {
	t.Parallel()

	// The same key supplied twice is one principal in the metadata, and
	// SignEnvelope replaces a signature whose key ID it already holds, so a
	// threshold of two is unsatisfiable however many times the key is repeated.
	tests := map[string]struct {
		mutate func(*BootstrapParams, ed25519.PublicKey, ed25519.PrivateKey)
		errMsg string
	}{
		"duplicate root signer": {
			mutate: func(p *BootstrapParams, pub ed25519.PublicKey, priv ed25519.PrivateKey) {
				p.RootSigners = []crypto.Signer{priv, priv}
				p.RootPublicKeys = []crypto.PublicKey{pub, pub}
				p.RootThreshold = 2
			},
			errMsg: "root threshold 2 exceeds the 1 distinct root public keys",
		},
		"distinct keys but duplicate signer": {
			mutate: func(p *BootstrapParams, pub ed25519.PublicKey, priv ed25519.PrivateKey) {
				otherPub, _ := newTestED25519(t)
				p.RootSigners = []crypto.Signer{priv, priv}
				p.RootPublicKeys = []crypto.PublicKey{pub, otherPub}
				p.RootThreshold = 2
			},
			errMsg: "root threshold 2 exceeds the 1 distinct root signers",
		},
		"duplicate RSL signer key": {
			mutate: func(p *BootstrapParams, _ ed25519.PublicKey, _ ed25519.PrivateKey) {
				rslPub := p.RSLSignerPublicKeys[0]
				p.RSLSignerPublicKeys = []crypto.PublicKey{rslPub, rslPub}
				p.RSLSignerThreshold = 2
			},
			errMsg: "RSL signer threshold 2 exceeds the 1 distinct RSL signer public keys",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			params, _ := testParams(t)
			rootPub, ok := params.RootPublicKeys[0].(ed25519.PublicKey)
			require.True(t, ok)
			rootPriv, ok := params.RootSigners[0].(ed25519.PrivateKey)
			require.True(t, ok)

			test.mutate(&params, rootPub, rootPriv)

			_, err := Bootstrap(context.Background(), store, params)
			require.ErrorContains(t, err, test.errMsg)

			for _, ref := range []string{Ref, StagingRef} {
				_, err := store.GetReference(ref)
				assert.ErrorIs(t, err, gitstore.ErrReferenceNotFound, "%s must not be written", ref)
			}
		})
	}
}

func TestBootstrapDeduplicatesRepeatedKeys(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, rslPriv := testParams(t)

	rootPub := params.RootPublicKeys[0]
	rootPriv := params.RootSigners[0]
	rslPub := params.RSLSignerPublicKeys[0]

	params.RootPublicKeys = []crypto.PublicKey{rootPub, rootPub}
	params.RootSigners = []crypto.Signer{rootPriv, rootPriv}
	params.RSLSignerPublicKeys = []crypto.PublicKey{rslPub, rslPub}

	_, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)

	rootID, err := PrincipalID(rootPub)
	require.NoError(t, err)
	rslID, err := PrincipalID(rslPub)
	require.NoError(t, err)

	info, err := InspectRoot(context.Background(), store)
	require.NoError(t, err)
	assert.Equal(t, []string{rootID}, info.RootKeyIDs)
	assert.Equal(t, []string{rslID}, info.AuthorizedRSLSigners)

	recordRefUpdate(t, store, rslPriv)
	_, err = VerifyRef(context.Background(), store, testRef)
	assert.NoError(t, err)
}

// unsupportedSigner is a crypto.Signer holding a key type SSH cannot represent.
type unsupportedSigner struct{}

func (unsupportedSigner) Public() crypto.PublicKey { return struct{}{} }

func (unsupportedSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return nil, errors.New("unsupported")
}

func TestBootstrapRejectsUnsupportedRootSigner(t *testing.T) {
	t.Parallel()

	params, _ := testParams(t)
	params.RootSigners = []crypto.Signer{unsupportedSigner{}}

	_, err := Bootstrap(context.Background(), newTestStore(t), params)
	assert.Error(t, err)
}

func TestBootstrapDefaultsRSLSignerThreshold(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, rslPriv := testParams(t)
	params.RSLSignerThreshold = 0

	_, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)

	recordRefUpdate(t, store, rslPriv)
	_, err = VerifyRef(context.Background(), store, testRef)
	assert.NoError(t, err, "an unset threshold must default to one and accept a single signature")
}

func TestRefConstantsMatchEngine(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ipolicy.PolicyRef, Ref)
	assert.Equal(t, ipolicy.PolicyStagingRef, StagingRef)
}

type faultyStore struct {
	gitstore.Storer

	failGetReference    bool
	failWriteTree       bool
	failKnowsCommit     bool
	failSetReferenceFor string

	// atFailure records which references existed at the moment the injected
	// fault fired, so a rollback assertion cannot pass because nothing was
	// ever written.
	atFailure map[string]bool
}

func (f faultyStore) SetReference(ref string, id githash.Hash) error {
	if f.failSetReferenceFor == ref {
		for _, observed := range []string{StagingRef, rsl.Ref} {
			if _, err := f.Storer.GetReference(observed); err == nil {
				f.atFailure[observed] = true
			}
		}

		return errFault
	}

	return f.Storer.SetReference(ref, id)
}

var errFault = errors.New("injected store fault")

func (f faultyStore) GetReference(ref string) (githash.Hash, error) {
	if f.failGetReference {
		return nil, errFault
	}

	return f.Storer.GetReference(ref)
}

func (f faultyStore) WriteTree(entries []gitstore.TreeEntry) (githash.Hash, error) {
	if f.failWriteTree {
		return nil, errFault
	}

	return f.Storer.WriteTree(entries)
}

func (f faultyStore) KnowsCommit(commitID, ancestorID githash.Hash) (bool, error) {
	if f.failKnowsCommit {
		return false, errFault
	}

	return f.Storer.KnowsCommit(commitID, ancestorID)
}

func TestBootstrapPropagatesStoreErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]faultyStore{
		"reference lookup fails": {failGetReference: true},
		"tree write fails":       {failWriteTree: true},
	}

	for name, fault := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			params, _ := testParams(t)
			fault.Storer = newTestStore(t)

			_, err := Bootstrap(context.Background(), fault, params)
			assert.ErrorIs(t, err, errFault)
		})
	}
}

func TestAuthorPropagatesTreeWriteError(t *testing.T) {
	t.Parallel()

	params, _ := testParams(t)
	store := faultyStore{Storer: newTestStore(t), failWriteTree: true}

	_, err := Author(context.Background(), store, params)
	assert.ErrorIs(t, err, errFault)
}

func TestVerifyTransitionPropagatesAncestryError(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, _ := testParams(t)

	oldTip, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)

	successor := params
	successor.RootVersion = 2
	tree, err := Author(context.Background(), store, successor)
	require.NoError(t, err)
	newTip, err := store.Commit(tree, Ref, "Transition gittuf policy", false)
	require.NoError(t, err)

	faulty := faultyStore{Storer: store, failKnowsCommit: true}
	err = VerifyTransition(context.Background(), faulty, oldTip, newTip)
	assert.ErrorIs(t, err, errFault)
}

func TestBootstrapRollsBackOnFailure(t *testing.T) {
	t.Parallel()

	// Applying the staged policy sets Ref. Failing that write leaves the
	// staging ref and the RSL entry Commit already wrote, which is the state
	// the rollback has to clear.
	store := newTestStore(t)
	params, _ := testParams(t)
	faulty := faultyStore{Storer: store, failSetReferenceFor: Ref, atFailure: map[string]bool{}}

	_, err := Bootstrap(context.Background(), faulty, params)
	require.ErrorIs(t, err, errFault)

	require.True(t, faulty.atFailure[StagingRef], "the staging ref must exist when the fault fires, or this test proves nothing")
	require.True(t, faulty.atFailure[rsl.Ref], "the RSL must exist when the fault fires, or this test proves nothing")

	for _, ref := range []string{Ref, StagingRef, rsl.Ref} {
		_, err := store.GetReference(ref)
		assert.ErrorIs(t, err, gitstore.ErrReferenceNotFound, "%s must not survive a failed bootstrap", ref)
	}
}

func TestBootstrapRetriesAfterRolledBackFailure(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, rslPriv := testParams(t)

	_, err := Bootstrap(context.Background(), faultyStore{Storer: store, failSetReferenceFor: Ref, atFailure: map[string]bool{}}, params)
	require.ErrorIs(t, err, errFault)

	// The whole point of rolling back: a transient fault must not strand the
	// repository behind the "already initialized" check.
	tip, err := Bootstrap(context.Background(), store, params)
	require.NoError(t, err)
	assert.False(t, tip.IsZero())

	recordRefUpdate(t, store, rslPriv)
	_, err = VerifyRef(context.Background(), store, testRef)
	assert.NoError(t, err)
}

func TestBootstrapRollbackPreservesExistingRSL(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	params, rslPriv := testParams(t)

	// An RSL that predates the bootstrap must be restored, not deleted.
	keyPEM := opensshPEM(t, rslPriv)
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, store, testRef, 1, keyPEM)
	require.NoError(t, rsl.NewReferenceEntry(testRef, commitIDs[0]).CommitUsingSpecificKey(store, keyPEM))

	originalRSLTip, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)

	_, err = Bootstrap(context.Background(), faultyStore{Storer: store, failSetReferenceFor: Ref, atFailure: map[string]bool{}}, params)
	require.ErrorIs(t, err, errFault)

	restoredRSLTip, err := store.GetReference(rsl.Ref)
	require.NoError(t, err)
	assert.Equal(t, originalRSLTip.String(), restoredRSLTip.String(), "the pre-existing RSL must be restored")

	for _, ref := range []string{Ref, StagingRef} {
		_, err := store.GetReference(ref)
		assert.ErrorIs(t, err, gitstore.ErrReferenceNotFound)
	}
}

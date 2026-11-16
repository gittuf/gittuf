// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	loadpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/loadpolicy"
	verifyopts "github.com/gittuf/gittuf/experimental/gittuf/options/verify"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/policy"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/gittuf/gittuf/pkg/rsl"
	gossh "golang.org/x/crypto/ssh"
)

// ErrRefNotFullyQualified is returned when a StorerRepository is given a
// reference name that does not start with refs/. Resolving short names needs
// the on-disk repository layout, which a Storer does not expose.
var ErrRefNotFullyQualified = errors.New("reference name must be fully qualified")

// ErrRefNotProtected is returned by Policy.Satisfies when no rule or global
// threshold rule protects the reference, so any principal may sign for it.
var ErrRefNotProtected = errors.New("no rule protects the reference")

// ErrPolicyNotSatisfied is returned by Policy.Satisfies when the principals do
// not meet the policy protecting the reference.
var ErrPolicyNotSatisfied = errors.New("principals do not satisfy the policy")

// StorerRepository verifies and inspects gittuf policy over any
// gitstore.Storer. Repository is bound to an on-disk Git repository. This type
// carries only the operations that need nothing beyond the Storer interface,
// so a caller can run them over its own storage backend.
type StorerRepository struct {
	store gitstore.Storer
}

// NewStorerRepository returns a StorerRepository reading from store.
func NewStorerRepository(store gitstore.Storer) *StorerRepository {
	return &StorerRepository{store: store}
}

// VerifyRef verifies refName against the applied policy and returns the tip
// the RSL attests for it. Unlike Repository.VerifyRef, it does not compare that
// tip with refName's current value in the store, so a caller can verify a tip
// it has not installed yet. refName must be fully qualified.
//
// With verify.WithOverrideRefName, the RSL entries for the override name are
// verified instead.
func (s *StorerRepository) VerifyRef(ctx context.Context, refName string, opts ...verifyopts.Option) (githash.Hash, error) {
	options := &verifyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	refName, err := rslRefName(refName, options)
	if err != nil {
		return nil, err
	}

	return verifyRef(ctx, s.store, refName, options.LatestOnly)
}

// VerifyRefFromEntry verifies refName against the applied policy starting at
// the RSL entry entryID rather than the first one, and returns the tip the RSL
// attests for it. As with VerifyRef, the tip is not compared with refName's
// current value in the store. Of the verify options, only
// verify.WithOverrideRefName applies.
func (s *StorerRepository) VerifyRefFromEntry(ctx context.Context, refName string, entryID githash.Hash, opts ...verifyopts.Option) (githash.Hash, error) {
	options := &verifyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	refName, err := rslRefName(refName, options)
	if err != nil {
		return nil, err
	}

	return policy.NewPolicyVerifier(s.store).VerifyRefFromEntry(ctx, refName, entryID)
}

// RootInfo reports facts about a root of trust.
type RootInfo struct {
	// Version is the root metadata version.
	Version uint64

	// RootKeyIDs are the principal IDs trusted for the root role.
	RootKeyIDs []string
}

// Policy is a verified policy loaded by LoadPolicy. It is not safe for
// concurrent use.
type Policy struct {
	state *policy.State
	root  RootInfo
}

// LoadPolicy loads the applied policy. Loading verifies the metadata signatures
// and every root transition up to the loaded entry, so unsigned or tampered
// metadata fails.
//
// It does not establish that the history is the one the caller expects: the
// first root in the RSL is trusted on sight. Pass
// loadpolicy.WithInitialRootKeys to anchor it.
//
// The policy is read from the RSL, not from the policy reference. Without
// options it is the latest policy entry. See loadpolicy.WithEntry. The RSL tip
// is read once and every later read uses it, so a concurrent rewrite cannot
// change the history part way through a load.
func (s *StorerRepository) LoadPolicy(ctx context.Context, opts ...loadpolicyopts.Option) (*Policy, error) {
	options := &loadpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	rslTip, err := s.store.GetReference(rsl.Ref)
	if err != nil {
		return nil, err
	}
	store := rslSnapshot{Storer: s.store, rslTip: rslTip}

	state, err := loadState(ctx, store, options)
	if err != nil {
		return nil, err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return nil, err
	}

	rootPrincipals, err := rootMetadata.GetRootPrincipals()
	if err != nil {
		return nil, err
	}

	rootKeyIDs := make([]string, 0, len(rootPrincipals))
	for _, principal := range rootPrincipals {
		rootKeyIDs = append(rootKeyIDs, principal.ID())
	}

	return &Policy{
		state: state,
		root: RootInfo{
			Version:    rootMetadata.GetVersion(),
			RootKeyIDs: rootKeyIDs,
		},
	}, nil
}

// Root reports the policy's root of trust.
func (p *Policy) Root() RootInfo {
	root := p.root
	root.RootKeyIDs = slices.Clone(p.root.RootKeyIDs)
	return root
}

// Principals returns the sorted IDs of every principal the policy declares, in
// its root and its rule files. A principal not listed cannot count toward any
// rule or global rule. Controller policies contribute only their global rules,
// which count these same principals.
func (p *Policy) Principals() []string {
	return slices.Sorted(maps.Keys(p.state.GetAllPrincipals()))
}

// Satisfies reports whether principalIDs, taken as the principals that sign for
// refName, meet the policy protecting it. It returns the name of the rule whose
// threshold they meet, or an empty name when only global threshold rules
// protect refName. refName must be fully qualified.
//
// The rule selection and thresholds are those verification applies, including
// delegated rule files, last trusted delegations, and global threshold rules.
// Approvals from attestations and GitHub apps are not considered, so a met
// policy never depends on them. Neither are checks that need a specific
// change: blocking force pushes, file rules, and for a tag reference the
// signature on the tag object itself. A tag whose object is unsigned fails
// verification even when Satisfies reports the policy met.
//
// ErrRefNotProtected is returned when nothing constrains who signs for refName,
// and ErrPolicyNotSatisfied when principalIDs fall short.
func (p *Policy) Satisfies(refName string, principalIDs ...string) (string, error) {
	if !strings.HasPrefix(refName, gitinterface.RefPrefix) {
		return "", fmt.Errorf("%w: %s", ErrRefNotFullyQualified, refName)
	}

	rule, err := p.state.SatisfiedBy("git:"+refName, set.NewSetFromItems(principalIDs...))
	switch {
	case errors.Is(err, policy.ErrTargetNotProtected):
		return "", fmt.Errorf("%w: %s", ErrRefNotProtected, refName)
	case errors.Is(err, policy.ErrVerifierConditionsUnmet):
		return "", fmt.Errorf("%w: %s", ErrPolicyNotSatisfied, refName)
	case err != nil:
		return "", err
	}

	return rule, nil
}

func loadState(ctx context.Context, store rslSnapshot, options *loadpolicyopts.Options) (*policy.State, error) {
	var loadOpts []policyopts.LoadStateOption
	if len(options.InitialRootKeys) != 0 {
		principals, err := principalsFromPublicKeys(options.InitialRootKeys)
		if err != nil {
			return nil, err
		}
		loadOpts = append(loadOpts, policyopts.WithInitialRootPrincipals(principals))
	}

	if !options.AtEntry {
		return policy.LoadCurrentState(ctx, store, policy.PolicyRef, loadOpts...)
	}

	// Refused rather than treated as absent. Falling back to the latest entry
	// would return a policy the caller did not ask for.
	if options.EntryID.IsZero() {
		return nil, fmt.Errorf("RSL entry ID must not be zero")
	}

	// GetEntry parses any commit that reads as an entry, including one no RSL
	// recorded, so membership is checked first.
	reachable, err := store.KnowsCommit(store.rslTip, options.EntryID)
	if err != nil {
		return nil, fmt.Errorf("unable to check RSL membership of %s: %w", options.EntryID.String(), err)
	}
	if !reachable {
		return nil, fmt.Errorf("RSL entry %s is not reachable from %s", options.EntryID.String(), rsl.Ref)
	}

	entry, err := rsl.GetEntry(store, options.EntryID)
	if err != nil {
		return nil, err
	}

	refEntry, ok := entry.(rsl.ReferenceUpdaterEntry)
	if !ok {
		return nil, fmt.Errorf("RSL entry %s is not a reference entry", options.EntryID.String())
	}

	if refEntry.GetRefName() != policy.PolicyRef {
		return nil, fmt.Errorf("RSL entry %s updates %q, not the policy ref %q", options.EntryID.String(), refEntry.GetRefName(), policy.PolicyRef)
	}

	return policy.LoadState(ctx, store, refEntry, loadOpts...)
}

// rslSnapshot reports a fixed RSL tip, so every read of the RSL through it sees
// the same history.
type rslSnapshot struct {
	gitstore.Storer
	rslTip githash.Hash
}

func (s rslSnapshot) GetReference(refName string) (githash.Hash, error) {
	if refName == rsl.Ref {
		return s.rslTip, nil
	}

	return s.Storer.GetReference(refName)
}

// rslRefName returns the reference whose RSL entries are verified.
func rslRefName(refName string, options *verifyopts.Options) (string, error) {
	if options.RefNameOverride != "" {
		refName = options.RefNameOverride
	}

	if !strings.HasPrefix(refName, gitinterface.RefPrefix) {
		return "", fmt.Errorf("%w: %s", ErrRefNotFullyQualified, refName)
	}

	return refName, nil
}

// verifyRef verifies refName against the applied policy and returns the tip
// the RSL attests for it. With latestOnly, only the latest RSL entry for
// refName is verified.
func verifyRef(ctx context.Context, store gitstore.Storer, refName string, latestOnly bool) (githash.Hash, error) {
	verifier := policy.NewPolicyVerifier(store)
	if latestOnly {
		return verifier.VerifyRef(ctx, refName)
	}

	return verifier.VerifyRefFull(ctx, refName)
}

// newPrincipal returns the gittuf principal for pub, identified by its SSH
// SHA-256 fingerprint.
func newPrincipal(pub crypto.PublicKey) (tuf.Principal, error) {
	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("unable to create ssh public key: %w", err)
	}

	verifier, err := ssh.NewVerifierFromKey(ssh.NewKeyFromPublicKey(sshPub))
	if err != nil {
		return nil, err
	}

	return tufv02.NewKeyFromSSLibKey(verifier.MetadataKey()), nil
}

// principalsFromPublicKeys returns the principals for pubs in order, keeping
// the first of each distinct principal ID.
func principalsFromPublicKeys(pubs []crypto.PublicKey) ([]tuf.Principal, error) {
	principals := make([]tuf.Principal, 0, len(pubs))
	seen := map[string]bool{}
	for _, pub := range pubs {
		principal, err := newPrincipal(pub)
		if err != nil {
			return nil, err
		}
		if seen[principal.ID()] {
			continue
		}
		seen[principal.ID()] = true
		principals = append(principals, principal)
	}

	return principals, nil
}

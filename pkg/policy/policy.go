// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Package policy exposes gittuf's policy and root-of-trust engine over any
// pkg/gitstore.Storer implementation. The engine is already parameterized by
// Storer, but experimental/gittuf.Repository, the only entry point reaching it,
// is hard-wired to an on-disk Git repository. This package lets an external
// module author, verify, and inspect gittuf policy over its own storage
// backend.
//
// Bootstrap authors a root of trust and a single top-level targets rule file
// authorizing a set of RSL signers over a set of ref patterns. Trust flows from
// the DSSE signatures over that metadata. Commits and RSL entries are Git-signed
// only when BootstrapParams.SignCommits is set, so a caller that leaves it unset
// is responsible for protecting the policy refs against rollback by other means.
package policy

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/gittuf/gittuf/pkg/rsl"
	gossh "golang.org/x/crypto/ssh"
)

const (
	// Ref is the Git reference holding the applied policy.
	Ref = policy.PolicyRef

	// StagingRef is the Git reference holding staged, not yet applied, policy.
	StagingRef = policy.PolicyStagingRef
)

var (
	// ErrMetadataRollbackDetected is returned by VerifyTransition when the new
	// state's metadata version is below the applied state's.
	ErrMetadataRollbackDetected = errors.New("policy metadata rollback detected")

	// ErrNotAncestor is returned by VerifyTransition when the new policy commit
	// does not descend from the applied one.
	ErrNotAncestor = errors.New("new policy does not descend from the applied policy")
)

// gitRulePrefix is the scheme a rule pattern uses to protect a Git reference.
// It must match internal/policy's gitReferenceRuleScheme.
const gitRulePrefix = "git:"

// rslSignerRuleName names the top-level targets rule authorizing the RSL
// signers over the requested ref patterns.
const rslSignerRuleName = "authorize-rsl-signers"

// BootstrapParams configures Bootstrap. Keys are passed as standard library
// types, so no exported field names a gittuf internal type.
type BootstrapParams struct {
	// RootSigners sign the root and top-level targets DSSE envelopes. Every
	// signer's public key must appear in RootPublicKeys, and there must be at
	// least RootThreshold of them, or the authored metadata cannot satisfy its
	// own thresholds.
	RootSigners []crypto.Signer

	// RootPublicKeys are the public keys trusted for the root role. They are
	// also trusted for the top-level targets role.
	RootPublicKeys []crypto.PublicKey

	// RootThreshold is the number of signatures required for the root and
	// top-level targets roles.
	RootThreshold int

	// RSLSignerPublicKeys are the public keys authorized to sign RSL entries
	// for RefPatterns.
	RSLSignerPublicKeys []crypto.PublicKey

	// RSLSignerThreshold is the number of RSL signatures required for a ref
	// matching RefPatterns. It defaults to 1 when unset.
	RSLSignerThreshold int

	// RefPatterns are the gittuf rule patterns the RSL signers are authorized
	// for, such as "git:refs/*".
	RefPatterns []string

	// Expires sets the expiry recorded in the root and targets metadata.
	Expires time.Time

	// RootVersion is the TUF metadata version stamped on the authored
	// metadata. It defaults to 1. gittuf rejects a policy transition whose
	// metadata version is below the applied one, so a profile re-authored over
	// an existing root must carry a version at least as high as the applied
	// root's. It is stamped on both the root and the top-level targets
	// metadata, which this package always authors together.
	RootVersion uint64

	// SignCommits Git-signs the policy commits and RSL entries using the
	// signing key configured on store.
	SignCommits bool
}

// Bootstrap authors the root of trust and the top-level targets rule file over
// store, staging and applying them to refs/gittuf/policy. It returns the
// applied policy commit ID.
func Bootstrap(ctx context.Context, store gitstore.Storer, p BootstrapParams) (githash.Hash, error) {
	state, err := authorState(ctx, p)
	if err != nil {
		return nil, err
	}

	for _, ref := range []string{Ref, StagingRef} {
		if _, err := store.GetReference(ref); err == nil {
			return nil, fmt.Errorf("policy already initialized on %s", ref)
		} else if !errors.Is(err, gitstore.ErrReferenceNotFound) {
			return nil, err
		}
	}

	// Bootstrap writes three references and the engine cannot undo them on a
	// first bootstrap: its reset paths restore a prior tip, and here there is
	// none. Without the rollback below, a failure part way through would leave
	// references that the check above then refuses to overwrite, so a transient
	// signing or backend fault would strand the repository with no way to retry.
	rslTip, hadRSL, err := referenceTip(store, rsl.Ref)
	if err != nil {
		return nil, err
	}

	if err := state.Commit(store, "Bootstrap gittuf policy", true, p.SignCommits); err != nil {
		return nil, rollbackBootstrap(store, rslTip, hadRSL, fmt.Errorf("unable to stage policy: %w", err))
	}

	if err := policy.Apply(ctx, store, p.SignCommits); err != nil {
		return nil, rollbackBootstrap(store, rslTip, hadRSL, fmt.Errorf("unable to apply policy: %w", err))
	}

	appliedTip, err := store.GetReference(Ref)
	if err != nil {
		return nil, rollbackBootstrap(store, rslTip, hadRSL, fmt.Errorf("unable to read applied policy reference: %w", err))
	}

	return appliedTip, nil
}

// referenceTip reports ref's tip and whether it exists.
func referenceTip(store gitstore.Storer, ref string) (githash.Hash, bool, error) {
	tip, err := store.GetReference(ref)
	if err != nil {
		if errors.Is(err, gitstore.ErrReferenceNotFound) {
			return nil, false, nil
		}

		return nil, false, err
	}

	return tip, true, nil
}

// rollbackBootstrap removes the references a failed Bootstrap created and puts
// the RSL back where it started, so the operation can be retried. Objects it
// wrote remain in the store, unreferenced and harmless. Cleanup failures are
// joined onto cause rather than hiding it.
func rollbackBootstrap(store gitstore.Storer, rslTip githash.Hash, hadRSL bool, cause error) error {
	errs := []error{cause}

	for _, ref := range []string{Ref, StagingRef} {
		if err := deleteReference(store, ref); err != nil {
			errs = append(errs, fmt.Errorf("unable to roll back %s: %w", ref, err))
		}
	}

	if hadRSL {
		if err := store.SetReference(rsl.Ref, rslTip); err != nil {
			errs = append(errs, fmt.Errorf("unable to roll back %s to %s: %w", rsl.Ref, rslTip.String(), err))
		}
	} else if err := deleteReference(store, rsl.Ref); err != nil {
		errs = append(errs, fmt.Errorf("unable to roll back %s: %w", rsl.Ref, err))
	}

	return errors.Join(errs...)
}

// deleteReference removes ref, treating an absent reference as success.
func deleteReference(store gitstore.Storer, ref string) error {
	if err := store.DeleteReference(ref); err != nil && !errors.Is(err, gitstore.ErrReferenceNotFound) {
		return err
	}

	return nil
}

// Author builds and signs the metadata for p and writes the resulting policy
// tree to store, returning the tree ID. It sets no reference and records no RSL
// entry, so the caller chooses where the tree is committed, what the commit's
// parent is, and how the write is recorded. Bootstrap is Author followed by the
// staging, RSL, and apply steps.
func Author(ctx context.Context, store gitstore.Storer, p BootstrapParams) (githash.Hash, error) {
	state, err := authorState(ctx, p)
	if err != nil {
		return nil, err
	}

	return state.WriteTree(store)
}

// VerifyTransition checks that the policy state at newTip is a legitimate
// successor to the one at oldTip: its root is signed by the keys the old state
// trusts for the root role at its threshold, its metadata versions do not
// regress (ErrMetadataRollbackDetected), and the new state is internally valid.
// Both commits must be present in store. It reads no reference and no RSL, so a
// caller can run it before installing newTip.
func VerifyTransition(ctx context.Context, store gitstore.Storer, oldTip, newTip githash.Hash) error {
	oldState, err := policy.LoadStateFromCommit(store, oldTip)
	if err != nil {
		return fmt.Errorf("unable to load policy state at %s: %w", oldTip.String(), err)
	}

	newState, err := policy.LoadStateFromCommit(store, newTip)
	if err != nil {
		return fmt.Errorf("unable to load policy state at %s: %w", newTip.String(), err)
	}

	if err := oldState.VerifyNewState(ctx, newState); err != nil {
		if errors.Is(err, policy.ErrMetadataRollbackDetected) {
			return fmt.Errorf("%w: %w", ErrMetadataRollbackDetected, err)
		}

		return err
	}

	if err := newState.Verify(ctx); err != nil {
		return err
	}

	descends, err := store.KnowsCommit(newTip, oldTip)
	if err != nil {
		return fmt.Errorf("unable to check policy ancestry: %w", err)
	}
	if !descends {
		return fmt.Errorf("%w: %s does not descend from %s", ErrNotAncestor, newTip.String(), oldTip.String())
	}

	return nil
}

// authorState validates p and returns the signed policy state it describes.
func authorState(ctx context.Context, p BootstrapParams) (*policy.State, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	// Thresholds are counted against distinct identities throughout. The same
	// key passed twice is one principal in the metadata, and SignEnvelope
	// replaces a signature carrying a key ID it already holds, so counting
	// slice entries would author metadata that cannot satisfy its own
	// thresholds.
	rootSigners, err := signersFromCryptoSigners(p.RootSigners)
	if err != nil {
		return nil, err
	}

	rootPrincipals, err := principalsFromPublicKeys(p.RootPublicKeys)
	if err != nil {
		return nil, err
	}

	rslPrincipals, err := principalsFromPublicKeys(p.RSLSignerPublicKeys)
	if err != nil {
		return nil, err
	}

	for _, rootSigner := range rootSigners {
		signerKeyID, err := rootSigner.KeyID()
		if err != nil {
			return nil, err
		}
		if !containsID(rootPrincipals, signerKeyID) {
			return nil, fmt.Errorf("root signer %s is not among the root public keys", signerKeyID)
		}
	}

	if len(rootPrincipals) < p.RootThreshold {
		return nil, fmt.Errorf("root threshold %d exceeds the %d distinct root public keys provided", p.RootThreshold, len(rootPrincipals))
	}
	if len(rootSigners) < p.RootThreshold {
		return nil, fmt.Errorf("root threshold %d exceeds the %d distinct root signers provided, so the authored metadata could not satisfy it", p.RootThreshold, len(rootSigners))
	}
	if len(rslPrincipals) < p.rslSignerThreshold() {
		return nil, fmt.Errorf("RSL signer threshold %d exceeds the %d distinct RSL signer public keys provided", p.rslSignerThreshold(), len(rslPrincipals))
	}

	rootMetadata, err := buildRootMetadata(p, rootPrincipals)
	if err != nil {
		return nil, err
	}

	rootEnv, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		return nil, err
	}
	for _, rootSigner := range rootSigners {
		rootEnv, err = dsse.SignEnvelope(ctx, rootEnv, rootSigner)
		if err != nil {
			return nil, err
		}
	}

	targetsMetadata, err := buildTargetsMetadata(p, rslPrincipals)
	if err != nil {
		return nil, err
	}

	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return nil, err
	}
	for _, rootSigner := range rootSigners {
		targetsEnv, err = dsse.SignEnvelope(ctx, targetsEnv, rootSigner)
		if err != nil {
			return nil, err
		}
	}

	return &policy.State{
		Metadata: &policy.StateMetadata{
			RootEnvelope:    rootEnv,
			TargetsEnvelope: targetsEnv,
		},
	}, nil
}

func (p BootstrapParams) validate() error {
	if len(p.RootSigners) == 0 {
		return fmt.Errorf("at least one root signer must be provided")
	}
	for _, signer := range p.RootSigners {
		if signer == nil {
			return fmt.Errorf("root signer must not be nil")
		}
	}
	if len(p.RootPublicKeys) == 0 {
		return fmt.Errorf("at least one root public key must be provided")
	}
	if p.RootThreshold < 1 {
		return fmt.Errorf("root threshold must be at least 1")
	}
	if len(p.RSLSignerPublicKeys) == 0 {
		return fmt.Errorf("at least one RSL signer public key must be provided")
	}
	if p.RSLSignerThreshold < 0 {
		return fmt.Errorf("RSL signer threshold must not be negative")
	}
	if len(p.RefPatterns) == 0 {
		return fmt.Errorf("at least one ref pattern must be provided")
	}
	if p.Expires.IsZero() {
		return fmt.Errorf("expiry must be provided")
	}

	return nil
}

// rslSignerThreshold is the configured RSL signer threshold, defaulting to 1.
func (p BootstrapParams) rslSignerThreshold() int {
	if p.RSLSignerThreshold == 0 {
		return 1
	}

	return p.RSLSignerThreshold
}

// PrincipalID returns the gittuf principal ID for a public key, the SSH SHA-256
// fingerprint. It lets a caller match its own keys against the IDs reported by
// InspectRoot.
func PrincipalID(pub crypto.PublicKey) (string, error) {
	principal, err := newPrincipal(pub)
	if err != nil {
		return "", err
	}

	return principal.ID(), nil
}

// VerifyRef verifies the latest RSL entry for ref against the applied policy on
// refs/gittuf/policy, returning the verified tip.
func VerifyRef(ctx context.Context, store gitstore.Storer, ref string) (githash.Hash, error) {
	return policy.NewPolicyVerifier(store).VerifyRef(ctx, ref)
}

// VerifyRefFromEntry verifies ref against the applied policy starting from the
// given RSL entry rather than the first one, returning the verified tip.
func VerifyRefFromEntry(ctx context.Context, store gitstore.Storer, ref string, entryID githash.Hash) (githash.Hash, error) {
	return policy.NewPolicyVerifier(store).VerifyRefFromEntry(ctx, ref, entryID)
}

// RootInfo reports facts about the applied root of trust.
type RootInfo struct {
	// Version is the root metadata version.
	Version uint64

	// RootKeyIDs are the principal IDs trusted for the root role.
	RootKeyIDs []string

	// AuthorizedRSLSigners are the principal IDs authorized by the rules in the
	// top-level targets rule file.
	AuthorizedRSLSigners []string
}

// InspectRoot loads the applied policy from refs/gittuf/policy and reports root
// facts. Loading verifies the metadata signatures against the trusted root, so
// this is a verified read: a load of tampered metadata fails.
func InspectRoot(ctx context.Context, store gitstore.Storer) (RootInfo, error) {
	state, err := policy.LoadCurrentState(ctx, store, Ref)
	if err != nil {
		return RootInfo{}, err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return RootInfo{}, err
	}

	rootPrincipals, err := rootMetadata.GetRootPrincipals()
	if err != nil {
		return RootInfo{}, err
	}

	signers, err := authorizedRSLSigners(state)
	if err != nil {
		return RootInfo{}, err
	}

	return RootInfo{
		Version:              rootMetadata.GetVersion(),
		RootKeyIDs:           principalIDs(rootPrincipals),
		AuthorizedRSLSigners: signers,
	}, nil
}

func buildRootMetadata(p BootstrapParams, rootPrincipals []tuf.Principal) (tuf.RootMetadata, error) {
	rootMetadata := tufv02.NewRootMetadata()
	rootMetadata.SetExpires(p.Expires.Format(time.RFC3339))
	if p.RootVersion > rootMetadata.Version {
		rootMetadata.Version = p.RootVersion
	}

	for _, principal := range rootPrincipals {
		if err := rootMetadata.AddRootPrincipal(principal); err != nil {
			return nil, err
		}
		if err := rootMetadata.AddPrimaryRuleFilePrincipal(principal); err != nil {
			return nil, err
		}
	}

	if err := rootMetadata.UpdateRootThreshold(p.RootThreshold); err != nil {
		return nil, err
	}
	if err := rootMetadata.UpdatePrimaryRuleFileThreshold(p.RootThreshold); err != nil {
		return nil, err
	}

	return rootMetadata, nil
}

func buildTargetsMetadata(p BootstrapParams, rslPrincipals []tuf.Principal) (tuf.TargetsMetadata, error) {
	targetsMetadata := tufv02.NewTargetsMetadata()
	targetsMetadata.SetExpires(p.Expires.Format(time.RFC3339))
	if p.RootVersion > targetsMetadata.Version {
		targetsMetadata.Version = p.RootVersion
	}

	ids := make([]string, 0, len(rslPrincipals))
	for _, principal := range rslPrincipals {
		if err := targetsMetadata.AddPrincipal(principal); err != nil {
			return nil, err
		}
		ids = append(ids, principal.ID())
	}

	if err := targetsMetadata.AddRule(rslSignerRuleName, ids, p.RefPatterns, p.rslSignerThreshold()); err != nil {
		return nil, err
	}

	return targetsMetadata, nil
}

// authorizedRSLSigners returns the deduplicated principal IDs named by the
// top-level rules that protect a Git reference namespace. A rule protecting
// only file namespaces authorizes file approvals, not RSL signatures, so its
// principals are not reported.
func authorizedRSLSigners(state *policy.State) ([]string, error) {
	if !state.HasTargetsRole(policy.TargetsRoleName) {
		return nil, nil
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	signers := []string{}
	for _, rule := range targetsMetadata.GetRules() {
		ids := rule.GetPrincipalIDs()
		if ids == nil || !protectsGitNamespace(rule) {
			continue
		}
		for _, id := range ids.Contents() {
			if seen[id] {
				continue
			}
			seen[id] = true
			signers = append(signers, id)
		}
	}

	return signers, nil
}

// protectsGitNamespace reports whether any of the rule's patterns protect a Git
// reference rather than only file paths.
func protectsGitNamespace(rule tuf.Rule) bool {
	for _, pattern := range rule.GetProtectedNamespaces() {
		if strings.HasPrefix(pattern, gitRulePrefix) {
			return true
		}
	}

	return false
}

func newPrincipal(pub crypto.PublicKey) (tuf.Principal, error) {
	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("unable to create ssh public key: %w", err)
	}

	key, err := ssh.NewVerifierFromKey(ssh.NewKeyFromPublicKey(sshPub))
	if err != nil {
		return nil, err
	}

	return tufv02.NewKeyFromSSLibKey(key.MetadataKey()), nil
}

// principalsFromPublicKeys builds the principals for pubs, in order, keeping
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

// signersFromCryptoSigners builds the DSSE signers for signers, in order,
// keeping the first of each distinct key ID.
func signersFromCryptoSigners(signers []crypto.Signer) ([]sslibdsse.SignerVerifier, error) {
	built := make([]sslibdsse.SignerVerifier, 0, len(signers))
	seen := map[string]bool{}
	for _, signer := range signers {
		rootSigner, err := ssh.NewSignerFromCryptoSigner(signer)
		if err != nil {
			return nil, err
		}
		keyID, err := rootSigner.KeyID()
		if err != nil {
			return nil, err
		}
		if seen[keyID] {
			continue
		}
		seen[keyID] = true
		built = append(built, rootSigner)
	}

	return built, nil
}

func principalIDs(principals []tuf.Principal) []string {
	ids := make([]string, 0, len(principals))
	for _, principal := range principals {
		ids = append(ids, principal.ID())
	}

	return ids
}

func containsID(principals []tuf.Principal, id string) bool {
	for _, principal := range principals {
		if principal.ID() == id {
			return true
		}
	}

	return false
}

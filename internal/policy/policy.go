// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/gittuf/gittuf/internal/common/set"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/internal/tuf/migrations"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
)

const (
	// PolicyRef defines the Git namespace used for gittuf policies.
	PolicyRef = "refs/gittuf/policy"

	// PolicyIndexRef defines the Git namespace used as the local scratchpad
	// for pending policy mutations (the analog of git's index — never pushed).
	// `gittuf policy stage` promotes envelopes from here into PolicyStagingRef.
	PolicyIndexRef = "refs/gittuf/policy-index"

	// PolicyStagingRef defines the Git namespace that holds the officially
	// proposed (and possibly co-signed) policy state. It sits between
	// PolicyIndexRef (local scratchpad) and PolicyRef (applied), and is
	// advanced by `gittuf policy stage`.
	PolicyStagingRef = "refs/gittuf/policy-staging"

	// RootRoleName defines the expected name for the gittuf root of trust.
	RootRoleName = "root"

	// TargetsRoleName defines the expected name for the top level gittuf policy file.
	TargetsRoleName = "targets"

	// DefaultCommitMessage defines the fallback message to use when updating the policy ref if an action specific message is unavailable.
	DefaultCommitMessage = "Update policy state"

	metadataTreeEntryName = "metadata"

	gitReferenceRuleScheme = "git"
	fileRuleScheme         = "file"
)

var (
	ErrMetadataNotFound              = errors.New("unable to find requested metadata file; has it been initialized?")
	ErrDanglingDelegationMetadata    = errors.New("unreachable targets metadata found")
	ErrPolicyNotFound                = errors.New("cannot find policy")
	ErrInvalidPolicy                 = errors.New("invalid policy state (is policy reference out of sync with corresponding RSL entry?)")
	ErrNotAncestor                   = errors.New("cannot apply changes since policy is not an ancestor of the policy staging")
	ErrControllerMetadataNotFound    = errors.New("requested controller repository metadata not found")
	ErrControllerMetadataNotVerified = errors.New("unable to verify controller repository metadata")
)

// State contains the full set of metadata and root keys present in a policy
// state.
type State struct {
	Metadata           *StateMetadata
	ControllerMetadata map[string]*StateMetadata

	Hooks map[tuf.HookStage][]tuf.Hook

	GitHubApps map[string]tuf.GitHubApp

	repository     *gitinterface.Repository
	loadedEntry    rsl.ReferenceUpdaterEntry
	verifiersCache map[string][]*SignatureVerifier
	ruleNames      *set.Set[string]
	allPrincipals  map[string]tuf.Principal
	hasFileRule    bool
	globalRules    map[string][]tuf.GlobalRule
}

type StateMetadata struct {
	RootEnvelope        *sslibdsse.Envelope
	TargetsEnvelope     *sslibdsse.Envelope
	DelegationEnvelopes map[string]*sslibdsse.Envelope
}

func (s *StateMetadata) WriteTree(repo *gitinterface.Repository) (gitinterface.Hash, error) {
	metadata := map[string]*sslibdsse.Envelope{}
	metadata[RootRoleName] = s.RootEnvelope
	if s.TargetsEnvelope != nil {
		metadata[TargetsRoleName] = s.TargetsEnvelope
	}

	if s.DelegationEnvelopes != nil {
		for k, v := range s.DelegationEnvelopes {
			metadata[k] = v
		}
	}

	allTreeEntries := []gitinterface.TreeEntry{}

	for name, env := range metadata {
		envContents, err := json.Marshal(env)
		if err != nil {
			return nil, err
		}

		blobID, err := repo.WriteBlob(envContents)
		if err != nil {
			return nil, err
		}

		allTreeEntries = append(allTreeEntries, gitinterface.NewEntryBlob(name+".json", blobID))
	}

	treeBuilder := gitinterface.NewTreeBuilder(repo)
	return treeBuilder.WriteTreeFromEntries(allTreeEntries)
}

// LoadState returns the State of the repository's policy corresponding to the
// entry. It verifies the root of trust for the state from the initial policy
// entry in the RSL. If no policy states are found and the entry is for the
// policy-staging ref, that entry is returned with no verification.
func LoadState(ctx context.Context, repo *gitinterface.Repository, requestedEntry rsl.ReferenceUpdaterEntry, opts ...policyopts.LoadStateOption) (*State, error) {
	// Regardless of whether we've been asked for policy ref or staging ref,
	// we want to examine and verify consecutive policy states that appear
	// before the entry. This is why we don't just load the state and return
	// if entry is for the staging ref.

	options := &policyopts.LoadStateOptions{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug(fmt.Sprintf("Loading policy at entry '%s'...", requestedEntry.GetID().String()))

	// TODO: should this searcher be inherited when invoked via Verifier?
	searcher := newSearcher(repo)

	slog.Debug("Finding first policy entry...")
	firstPolicyEntry, err := searcher.FindFirstPolicyEntry()
	if err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			// we don't have a policy entry yet
			// we just return the state for the requested entry
			slog.Debug("No applied policy found, loading requested policy without further verification...")
			return loadStateForEntry(repo, requestedEntry)
		}
		return nil, err
	}

	if firstPolicyEntry.GetID().Equal(requestedEntry.GetID()) {
		slog.Debug("Requested policy's entry is the same as first policy entry")
		state, err := loadStateForEntry(repo, requestedEntry)
		if err != nil {
			return nil, err
		}

		if len(options.InitialRootPrincipals) == 0 {
			slog.Debug(fmt.Sprintf("Trusting root of trust for initial policy '%s'...", firstPolicyEntry.GetID().String()))
			return state, nil
		}

		slog.Debug("Verifying root of trust using provided initial root principals...")
		verifier := &SignatureVerifier{
			repository: repo,
			name:       "initial-root-verifier",
			principals: options.InitialRootPrincipals,
			threshold:  len(options.InitialRootPrincipals),
		}

		_, err = verifier.Verify(ctx, nil, state.Metadata.RootEnvelope)
		return state, err
	}

	// check if firstPolicyEntry is **after** requested entry
	// this can happen when the requested entry is for policy-staging before
	// Apply() was ever called
	slog.Debug("Checking if first policy entry was after requested policy's entry...")
	knows, err := repo.KnowsCommit(firstPolicyEntry.GetID(), requestedEntry.GetID())
	if err != nil {
		return nil, err
	}
	if knows {
		// the first policy entry knows the requested entry, meaning the
		// requested entry is an ancestor of the first policy entry
		// we just return the state for the requested entry
		slog.Debug("Requested policy's entry was before first applied policy, loading requested policy without verification...")
		return loadStateForEntry(repo, requestedEntry)
	}

	// If requestedEntry.RefName == policy, then allPolicyEntries includes requestedEntry
	// If requestedEntry.RefName == policy-staging, then allPolicyEntries does not include requestedEntry
	slog.Debug("Finding all policies between first policy and requested policy...")
	allPolicyEntries, err := searcher.FindPolicyEntriesInRange(firstPolicyEntry, requestedEntry)
	if err != nil {
		return nil, err
	}

	// We load the very first policy entry with no additional verification,
	// the root keys are implicitly trusted
	initialPolicyState, err := loadStateForEntry(repo, firstPolicyEntry)
	if err != nil {
		return nil, err
	}
	if len(options.InitialRootPrincipals) == 0 {
		slog.Debug(fmt.Sprintf("Trusting root of trust for initial policy '%s'...", firstPolicyEntry.GetID().String()))
	} else {
		slog.Debug("Verifying root of trust using provided initial root principals...")
		verifier := &SignatureVerifier{
			repository: repo,
			name:       "initial-root-verifier",
			principals: options.InitialRootPrincipals,
			threshold:  len(options.InitialRootPrincipals),
		}

		_, err = verifier.Verify(ctx, nil, initialPolicyState.Metadata.RootEnvelope)
		if err != nil {
			return nil, err
		}
	}

	verifiedState := initialPolicyState
	for _, entry := range allPolicyEntries[1:] {
		if entry.GetRefName() != PolicyRef {
			// The searcher _may_ include refs/gittuf/attestations
			// etc. which should be skipped
			continue
		}

		underTestState, err := loadStateForEntry(repo, entry)
		if err != nil {
			return nil, err
		}

		slog.Debug(fmt.Sprintf("Verifying root of trust for policy '%s'...", entry.GetID().String()))
		if err := verifiedState.VerifyNewState(ctx, underTestState); err != nil {
			return nil, fmt.Errorf("unable to verify roots of trust for policy states: %w", err)
		}

		verifiedState = underTestState
	}

	if requestedEntry.GetRefName() == PolicyRef {
		// We've already loaded it and done successive verification as
		// it was included in allPolicyEntries
		// This state is stored in verifiedState, we can do an internal
		// verification check and return

		slog.Debug("Validating requested policy's state...")
		if err := verifiedState.Verify(ctx); err != nil {
			return nil, fmt.Errorf("requested state has invalidly signed metadata: %w", err)
		}

		slog.Debug(fmt.Sprintf("Successfully loaded policy at entry '%s'!", requestedEntry.GetID().String()))
		return verifiedState, nil
	}

	// This is reached when requestedEntry is for staging ref
	// We've checked that all the policy states prior to this staging entry
	// are good (with their root of trust)
	return loadStateForEntry(repo, requestedEntry)
}

// LoadCurrentState returns the State corresponding to the repository's current
// active policy. It verifies the root of trust for the state starting from the
// initial policy entry in the RSL.
func LoadCurrentState(ctx context.Context, repo *gitinterface.Repository, ref string, opts ...policyopts.LoadStateOption) (*State, error) {
	options := &policyopts.LoadStateOptions{}
	for _, fn := range opts {
		fn(options)
	}

	if options.BypassRSL {
		commitID, err := repo.GetReference(ref)
		if err != nil {
			return nil, err
		}

		// Note: this will not set the loadedEntry field in the policy state
		return loadStateFromCommit(repo, commitID)
	}

	entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo, rsl.ForReference(ref))
	if err != nil {
		return nil, err
	}

	return LoadState(ctx, repo, entry, opts...)
}

// LoadFirstState returns the State corresponding to the repository's first
// active policy. It does not verify the root of trust since it is the initial policy.
func LoadFirstState(ctx context.Context, repo *gitinterface.Repository, opts ...policyopts.LoadStateOption) (*State, error) {
	firstEntry, _, err := rsl.GetFirstReferenceUpdaterEntryForRef(repo, PolicyRef)
	if err != nil {
		return nil, err
	}

	return LoadState(ctx, repo, firstEntry, opts...)
}

// FindVerifiersForPath identifies the trusted set of verifiers for the
// specified path. While walking the delegation graph for the path, signatures
// for delegated metadata files are verified using the verifier context.
func (s *State) FindVerifiersForPath(path string) ([]*SignatureVerifier, error) {
	if s.verifiersCache == nil {
		slog.Debug("Initializing path cache in policy...")
		s.verifiersCache = map[string][]*SignatureVerifier{}
	} else if verifiers, cacheHit := s.verifiersCache[path]; cacheHit {
		// Cache hit for this path in this policy
		slog.Debug(fmt.Sprintf("Found cached verifiers for path '%s'", path))
		return verifiers, nil
	}

	allVerifiers := []*SignatureVerifier{}

	if len(s.globalRules) != 0 {
		slog.Debug("Global constraints found, including exhaustive verifier...")
		// This has to go first so it's prioritized during verification
		// At least one global rule exists, return an exhaustive verifier
		verifier := &SignatureVerifier{
			repository: s.repository,
			name:       tuf.ExhaustiveVerifierName,
			principals: []tuf.Principal{}, // we'll add all principals below

			// threshold doesn't matter since we set verifyExhaustively to true
			threshold:          1,
			verifyExhaustively: true, // very important!
		}

		for _, principal := range s.allPrincipals {
			verifier.principals = append(verifier.principals, principal)
		}

		allVerifiers = append(allVerifiers, verifier)
	}

	specificVerifiers, err := s.findVerifiersForPathIfProtected(path)
	if err != nil {
		return nil, err
	}
	allVerifiers = append(allVerifiers, specificVerifiers...)

	// Note: we could loop through all global constraints and create a
	// verifier with all principals but targeting a specific constraint (or
	// an aggregate constraint that has the highest threshold requirement of
	// all the constraints that match path). However, this probably paints
	// us into a corner (only threshold requirements between two constraints
	// can be compared, we may have uncomparable constraints later), and we
	// would also want to verify every applicable global constraint for
	// safety, so we would be doing extra work for no reason.

	// add to cache
	s.verifiersCache[path] = allVerifiers
	// return verifiers
	return allVerifiers, nil
}

func (s *State) findVerifiersForPathIfProtected(path string) ([]*SignatureVerifier, error) {
	if !s.HasTargetsRole(TargetsRoleName) {
		// No policies exist
		return nil, ErrMetadataNotFound
	}

	// This envelope is verified when state is loaded, as this is
	// the start for all delegation graph searches
	targetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName, true) // migrating is fine since this is purely a query, let's start using tufv02 metadata
	if err != nil {
		return nil, err
	}

	allPrincipals := targetsMetadata.GetPrincipals()
	// each entry is a list of delegations from a particular metadata file
	groupedDelegations := [][]tuf.Rule{
		targetsMetadata.GetRules(),
	}

	seenRoles := map[string]bool{TargetsRoleName: true}

	var currentDelegationGroup []tuf.Rule
	verifiers := []*SignatureVerifier{}
	for {
		if len(groupedDelegations) == 0 {
			return verifiers, nil
		}

		currentDelegationGroup = groupedDelegations[0]
		groupedDelegations = groupedDelegations[1:]

		for len(currentDelegationGroup) > 1 {
			// Exit condition: Only allow rule found in the current group
			// => len(currentDelegationGroup) <= 1

			delegation := currentDelegationGroup[0]
			currentDelegationGroup = currentDelegationGroup[1:]

			if delegation.Matches(path) {
				verifier := &SignatureVerifier{
					repository: s.repository,
					name:       delegation.ID(),
					principals: make([]tuf.Principal, 0, delegation.GetPrincipalIDs().Len()),
					threshold:  delegation.GetThreshold(),
				}
				for _, principalID := range delegation.GetPrincipalIDs().Contents() {
					verifier.principals = append(verifier.principals, allPrincipals[principalID])
				}
				verifiers = append(verifiers, verifier)

				if _, seen := seenRoles[delegation.ID()]; seen {
					continue
				}

				if s.HasTargetsRole(delegation.ID()) {
					delegatedMetadata, err := s.GetTargetsMetadata(delegation.ID(), true) // migrating is fine since this is purely a query, let's start using tufv02 metadata
					if err != nil {
						return nil, err
					}

					seenRoles[delegation.ID()] = true

					for principalID, principal := range delegatedMetadata.GetPrincipals() {
						allPrincipals[principalID] = principal
					}

					// Add the current metadata's further delegations upfront to
					// be depth-first
					groupedDelegations = append([][]tuf.Rule{delegatedMetadata.GetRules()}, groupedDelegations...)

					if delegation.IsLastTrustedInRuleFile() {
						// Stop processing current delegation group, but proceed
						// with other groups
						break
					}
				}
			}
		}
	}
}

func (s *State) GetAllPrincipals() map[string]tuf.Principal {
	return s.allPrincipals
}

// Verify verifies the contents of the State for internal consistency.
// Specifically, it checks that the root keys in the root role match the ones
// stored on disk in the state. Further, it also verifies the signatures of the
// top level Targets role and all reachable delegated Targets roles. Any
// unreachable role returns an error.
func (s *State) Verify(ctx context.Context) error {
	rootVerifier, err := s.getRootVerifier()
	if err != nil {
		return err
	}

	if _, err := rootVerifier.Verify(ctx, gitinterface.ZeroHash, s.Metadata.RootEnvelope); err != nil {
		return err
	}

	// Check GitHub app approvals
	rootMetadata, err := s.GetRootMetadata(false) // don't migrate: this may be for a write and we don't want to write tufv02 metadata yet
	if err != nil {
		return err
	}
	githubAppEntries, err := rootMetadata.GetGitHubAppEntries()
	if err != nil {
		return err
	}
	for appName := range githubAppEntries {
		// TODO: retire IsGitHubAppApprovalTrusted
		if rootMetadata.IsGitHubAppApprovalTrusted(appName) {
			// Check that the GitHub app role is declared
			_, err := rootMetadata.GetGitHubAppPrincipals(appName)
			if err != nil {
				return err
			}
		}
	}

	// Check top-level targets and delegations
	if s.Metadata.TargetsEnvelope != nil {
		targetsVerifier, err := s.getTargetsVerifier()
		if err != nil {
			return err
		}

		if _, err := targetsVerifier.Verify(ctx, gitinterface.ZeroHash, s.Metadata.TargetsEnvelope); err != nil {
			return err
		}

	}

	if s.loadedEntry == nil {
		slog.Debug("Policy not loaded from RSL, skipping verification of controller metadata...")
		return nil
	}

	// Check controller root metadata
	if len(s.ControllerMetadata) != 0 {
		controllerRepositories := rootMetadata.GetControllerRepositories()
		for _, controllerRepositoryDetail := range controllerRepositories {
			controllerName := controllerRepositoryDetail.GetName()

			tmpDir, err := os.MkdirTemp("", fmt.Sprintf("gittuf-controller-%s-", controllerName))
			if err != nil {
				return fmt.Errorf("unable to clone controller repository: %w", err)
			}
			defer os.RemoveAll(tmpDir) //nolint:errcheck

			controllerRepository, err := gitinterface.CloneAndFetchRepository(controllerRepositoryDetail.GetLocation(), tmpDir, "", []string{PolicyRef, rsl.Ref}, true)
			if err != nil {
				return fmt.Errorf("unable to clone controller repository: %w", err)
			}

			// We need to LoadState() the state from which the root is derived
			// For that, we need to know when it was propagated into this repository
			upstreamEntryID := gitinterface.ZeroHash
			if entry, isPropagationEntry := s.loadedEntry.(*rsl.PropagationEntry); isPropagationEntry {
				// Check this entry
				if entry.RefName == PolicyRef && entry.UpstreamRepository == controllerRepositoryDetail.GetLocation() {
					upstreamEntryID = entry.UpstreamEntryID
				}
			}
			if upstreamEntryID.IsZero() {
				// not found yet
				// find propagation entry in local repo
				propagationEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(s.repository, rsl.BeforeEntryID(s.loadedEntry.GetID()), rsl.IsPropagationEntryForRepository(controllerRepositoryDetail.GetLocation()), rsl.ForReference(PolicyRef))
				if err != nil {
					return fmt.Errorf("%w, unable to verify controller repository: %w", ErrControllerMetadataNotVerified, err)
				}
				// We know propagationEntry is of this type because of the rsl.IsPropagationEntryForReference opt
				upstreamEntryID = propagationEntry.(*rsl.PropagationEntry).UpstreamEntryID
			}

			upstreamEntry, err := rsl.GetEntry(controllerRepository, upstreamEntryID)
			if err != nil {
				return err
			}

			// LoadState does full verification up until the requested entry
			if _, err := LoadState(ctx, controllerRepository, upstreamEntry.(rsl.ReferenceUpdaterEntry), policyopts.WithInitialRootPrincipals(controllerRepositoryDetail.GetInitialRootPrincipals())); err != nil {
				return fmt.Errorf("%w, unable to verify root of trust for controller '%s': %w", ErrControllerMetadataNotVerified, controllerName, err)
			}

			// TODO: verify git tree ID in upstream matches propagated
		}
	}

	return nil
}

// writeTree builds the full policy tree (metadata + controller metadata + hooks)
// and returns its tree ID. This is the shared tree-building portion of both
// Commit (which writes to PolicyIndexRef) and commitToRef (which writes to an
// arbitrary policy ref like PolicyStagingRef or PolicyRef).
func (s *State) writeTree(repo *gitinterface.Repository) (gitinterface.Hash, error) {
	allTreeEntries := []gitinterface.TreeEntry{}

	stateMetadataTreeID, err := s.Metadata.WriteTree(repo)
	if err != nil {
		return nil, err
	}
	allTreeEntries = append(allTreeEntries, gitinterface.NewEntryTree(metadataTreeEntryName, stateMetadataTreeID))

	for absoluteControllerPath, metadata := range s.ControllerMetadata {
		stateMetadataTreeID, err := metadata.WriteTree(repo)
		if err != nil {
			return nil, err
		}
		allTreeEntries = append(allTreeEntries, gitinterface.NewEntryTree(fmt.Sprintf("%s/%s", tuf.GittufControllerPrefix, absoluteControllerPath), stateMetadataTreeID))
	}

	for stage, hookSet := range s.Hooks {
		for _, hook := range hookSet {
			hookPath := fmt.Sprintf("%s/%s/%s", tuf.HooksPrefix, stage.String(), hook.ID())
			allTreeEntries = append(allTreeEntries, gitinterface.NewEntryBlob(hookPath, hook.GetBlobID()))
		}
	}

	treeBuilder := gitinterface.NewTreeBuilder(repo)
	return treeBuilder.WriteTreeFromEntries(allTreeEntries)
}

// commitToRef writes the State as a new commit on the given ref, optionally
// creating an RSL entry for it. If the ref already exists and an RSL entry
// commit fails, the ref is reset to its original tip.
func (s *State) commitToRef(repo *gitinterface.Repository, ref, commitMessage string, createRSLEntry, signCommit bool) (gitinterface.Hash, error) {
	if len(commitMessage) == 0 {
		commitMessage = DefaultCommitMessage
	}

	policyRootTreeID, err := s.writeTree(repo)
	if err != nil {
		return nil, err
	}

	originalCommitID, err := repo.GetReference(ref)
	if err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return nil, err
		}
	}

	commitID, err := repo.Commit(policyRootTreeID, ref, commitMessage, signCommit)
	if err != nil {
		return nil, err
	}

	// We must reset to the original commit if RSL entry creation fails from here onwards.
	if createRSLEntry {
		if err := rsl.NewReferenceEntry(ref, commitID).Commit(repo, signCommit); err != nil {
			if !originalCommitID.IsZero() {
				return nil, repo.ResetDueToError(err, ref, originalCommitID)
			}

			return nil, err
		}
	}

	return commitID, nil
}

// Commit writes the State to PolicyIndexRef — the local-only scratchpad for
// pending policy mutations. createRSLEntry must be false: PolicyIndexRef is
// never recorded in the RSL because it is purely local (the RSL is a shared,
// pushable log of officially proposed/applied state changes only). Callers
// that need to promote a mutation directly into the official proposal should
// also call CommitToStaging on a State independently loaded from
// PolicyStagingRef.
func (s *State) Commit(repo *gitinterface.Repository, commitMessage string, createRSLEntry, signCommit bool) error {
	if createRSLEntry {
		return fmt.Errorf("cannot create RSL entry for %s: it is a local-only scratchpad — call CommitToStaging on a State loaded from %s instead", PolicyIndexRef, PolicyStagingRef)
	}
	_, err := s.commitToRef(repo, PolicyIndexRef, commitMessage, false, signCommit)
	return err
}

// CommitToStaging writes the State to PolicyStagingRef and records a matching
// RSL entry. Used by mutations that bypass the PolicyIndexRef → stage workflow
// (typically root-of-trust mutations invoked with WithRSLEntry). The caller
// must have loaded the State from PolicyStagingRef and applied the mutation
// against that load — writing a State loaded from PolicyIndexRef would leak
// any pending index-only mutations into the official proposal.
func (s *State) CommitToStaging(repo *gitinterface.Repository, commitMessage string, signCommit bool) error {
	_, err := s.commitToRef(repo, PolicyStagingRef, commitMessage, true, signCommit)
	return err
}

// BuildOverlayState returns a new *State whose Metadata starts from base and,
// for each selected target name, replaces the corresponding envelope with the
// one from source. It performs no validation — neither signature verification
// nor coupling/reachability checks. The resulting state is verified later at
// apply time via State.Verify, which catches both signature failures (when
// thresholds aren't met) and dangling delegation envelopes
// (ErrDanglingDelegationMetadata).
//
// Selectors "root" and "targets" map to RootEnvelope and TargetsEnvelope
// respectively; any other name maps to DelegationEnvelopes[name]. A selected
// name absent from source while present in base results in removal of that
// envelope from the new state.
func BuildOverlayState(_ context.Context, base, source *State, selected []string) (*State, error) {
	if base == nil || base.Metadata == nil {
		return nil, fmt.Errorf("BuildOverlayState: base state has no metadata")
	}
	if source == nil || source.Metadata == nil {
		return nil, fmt.Errorf("BuildOverlayState: source state has no metadata")
	}

	newMetadata := &StateMetadata{
		RootEnvelope:    base.Metadata.RootEnvelope,
		TargetsEnvelope: base.Metadata.TargetsEnvelope,
	}
	if base.Metadata.DelegationEnvelopes != nil {
		newMetadata.DelegationEnvelopes = make(map[string]*sslibdsse.Envelope, len(base.Metadata.DelegationEnvelopes))
		for k, v := range base.Metadata.DelegationEnvelopes {
			newMetadata.DelegationEnvelopes[k] = v
		}
	}

	for _, name := range selected {
		switch name {
		case RootRoleName:
			if source.Metadata.RootEnvelope == nil {
				return nil, fmt.Errorf("BuildOverlayState: source has no %q envelope to overlay", name)
			}
			newMetadata.RootEnvelope = source.Metadata.RootEnvelope
		case TargetsRoleName:
			if source.Metadata.TargetsEnvelope == nil {
				return nil, fmt.Errorf("BuildOverlayState: source has no %q envelope to overlay", name)
			}
			newMetadata.TargetsEnvelope = source.Metadata.TargetsEnvelope
		default:
			srcEnv, srcHas := source.Metadata.DelegationEnvelopes[name]
			_, baseHas := newMetadata.DelegationEnvelopes[name]
			if !srcHas && !baseHas {
				return nil, fmt.Errorf("BuildOverlayState: no envelope named %q in either base or source", name)
			}
			if !srcHas {
				delete(newMetadata.DelegationEnvelopes, name)
				continue
			}
			if newMetadata.DelegationEnvelopes == nil {
				newMetadata.DelegationEnvelopes = map[string]*sslibdsse.Envelope{}
			}
			newMetadata.DelegationEnvelopes[name] = srcEnv
		}
	}

	return &State{
		Metadata:           newMetadata,
		ControllerMetadata: base.ControllerMetadata,
		Hooks:              base.Hooks,
		GitHubApps:         base.GitHubApps,
		repository:         base.repository,
	}, nil
}

// StageOverlayCommit commits the given state to PolicyStagingRef and records an
// RSL entry. If PolicyStagingRef doesn't exist yet, it is initialized from
// PolicyRef (or the new commit becomes a root commit if PolicyRef is also
// absent — the initial-policy bootstrap path). Returns the new staged tip.
func StageOverlayCommit(repo *gitinterface.Repository, state *State, message string, signCommit bool) (gitinterface.Hash, error) {
	if _, err := repo.GetReference(PolicyStagingRef); err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return nil, err
		}
		// PolicyStagingRef doesn't exist — initialize it from PolicyRef so the
		// new commit is properly parented. If PolicyRef is also missing
		// (initial policy bootstrap), commitToRef will produce a root commit.
		if policyTip, perr := repo.GetReference(PolicyRef); perr == nil {
			if err := repo.SetReference(PolicyStagingRef, policyTip); err != nil {
				return nil, err
			}
		}
	}
	return state.commitToRef(repo, PolicyStagingRef, message, true, signCommit)
}

// Apply takes valid changes from the policy staged ref, and fast-forward
// merges it into the policy ref. Apply only takes place if the latest state on
// the policy staged ref is valid. This prevents invalid changes to the policy
// taking effect, and allows new changes that until signed by multiple users
// would be invalid to be made, by utilizing the policy staged ref.
//
// PolicyIndexRef (the local scratchpad) is not consulted here — only
// PolicyStagingRef (the officially proposed policy) is promoted into PolicyRef.
func Apply(ctx context.Context, repo *gitinterface.Repository, signRSLEntry bool) error {
	// First, reconcile staged and staging with policy
	if err := ReconcileStaging(repo, signRSLEntry); err != nil {
		return err
	}

	// Get the reference for the PolicyRef
	referenceFound := true
	policyTip, err := repo.GetReference(PolicyRef)
	if err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return fmt.Errorf("failed to get policy reference %s: %w", PolicyRef, err)
		}
		referenceFound = false
	}

	entryFound := true
	policyEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo, rsl.ForReference(PolicyRef))
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return fmt.Errorf("failed to get policy RSL entry: %w", err)
		}

		entryFound = false
	}

	// case 1: both found -> verify tip matches entry
	// case 2: only one found -> return error
	// case 3: neither found -> nothing to verify
	switch {
	case referenceFound && entryFound:
		if !policyEntry.GetTargetID().Equal(policyTip) {
			return ErrInvalidPolicy
		}
	case (referenceFound && !entryFound) || (!referenceFound && entryFound):
		return ErrInvalidPolicy
	default:
		slog.Debug("No prior applied policy found")
		// Nothing to check or return here
	}

	// Get the reference for the PolicyStagingRef — the source of truth for what
	// Apply promotes. If no staged ref exists (nothing has been staged yet),
	// fall back to PolicyIndexRef for backward compatibility with the
	// initial-bootstrap path.
	stagedRef := PolicyStagingRef
	policyStagedTip, err := repo.GetReference(stagedRef)
	if err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return fmt.Errorf("failed to get policy staged reference %s: %w", stagedRef, err)
		}
		// Fall back to staging for bootstrap (initial policy is applied directly
		// from the staging ref because no stage call has happened yet).
		stagedRef = PolicyIndexRef
		policyStagedTip, err = repo.GetReference(stagedRef)
		if err != nil {
			return fmt.Errorf("failed to get policy staging reference %s: %w", stagedRef, err)
		}
	}

	// Check if the staged tip is ahead of PolicyRef (fast-forward)
	if !policyTip.IsZero() {
		// This check ensures that the policy staged branch is a direct forward progression of the policy branch,
		// preventing any overwrites of policy history and maintaining a linear policy evolution, since a
		// fast-forward merge does not work with a non-linear history.

		// This is only being checked if there are no problems finding the tip of the policy ref, since if there
		// is no tip, then it cannot be an ancestor of the tip of the policy staged ref
		isAncestor, err := repo.KnowsCommit(policyStagedTip, policyTip)
		if err != nil {
			return fmt.Errorf("failed to check if policy commit is ancestor of policy staged commit: %w", err)
		}
		if !isAncestor {
			return ErrNotAncestor
		}
	}

	// using LoadCurrentState to load and verify if the staged ref's
	// latest state is valid
	state, err := LoadCurrentState(ctx, repo, stagedRef)
	if err != nil {
		return fmt.Errorf("failed to load current state: %w", err)
	}
	if err := state.Verify(ctx); err != nil {
		return fmt.Errorf("staged policy is invalid: %w", err)
	}

	// Update the reference for the base to point to the new commit
	if err := repo.SetReference(PolicyRef, policyStagedTip); err != nil {
		return fmt.Errorf("failed to set new policy reference: %w", err)
	}

	if err := rsl.NewReferenceEntry(PolicyRef, policyStagedTip).Commit(repo, signRSLEntry); err != nil {
		if !policyTip.IsZero() {
			return repo.ResetDueToError(err, PolicyRef, policyTip)
		}

		return err
	}

	return nil
}

// Discard resets both PolicyStagingRef and PolicyIndexRef to PolicyRef. If
// PolicyRef does not exist, both staging refs are deleted instead.
func Discard(repo *gitinterface.Repository) error {
	policyTip, err := repo.GetReference(PolicyRef)
	if err != nil {
		if errors.Is(err, gitinterface.ErrReferenceNotFound) {
			for _, ref := range []string{PolicyStagingRef, PolicyIndexRef} {
				if err := repo.DeleteReference(ref); err != nil && !errors.Is(err, gitinterface.ErrReferenceNotFound) {
					return fmt.Errorf("failed to delete %s: %w", ref, err)
				}
			}
			return nil
		}
		return fmt.Errorf("failed to get policy reference %s: %w", PolicyRef, err)
	}

	for _, ref := range []string{PolicyStagingRef, PolicyIndexRef} {
		if err := repo.SetReference(ref, policyTip); err != nil {
			return fmt.Errorf("failed to reset %s: %w", ref, err)
		}
	}
	return nil
}

// ReconcileStaging walks the three-ref topology
// (PolicyIndexRef ⊇ PolicyStagingRef ⊇ PolicyRef) and brings out-of-date refs
// back into a fast-forward chain. Each ref rebases against the one directly
// below it; controller propagations into PolicyRef are absorbed up the chain.
func ReconcileStaging(repo *gitinterface.Repository, signCommit bool) error {
	// Step 1: Reconcile PolicyStagingRef vs PolicyRef.
	// Controller propagation can update PolicyRef directly; the staged
	// proposal may need the same controller changes to remain valid.
	if err := reconcileRefAgainstBase(repo, PolicyStagingRef, PolicyRef, signCommit); err != nil {
		return err
	}

	// Step 2: Auto-stage if there's no pending proposal but PolicyIndexRef
	// has moved. This preserves the legacy "mutate → apply" workflow: when
	// PolicyStagingRef equals PolicyRef (no explicit proposal in flight) and
	// PolicyIndexRef is a strict descendant of PolicyStagingRef, fast-forward
	// PolicyStagingRef to PolicyIndexRef. When PolicyStagingRef is already
	// ahead of PolicyRef (i.e., a user did an explicit `gittuf policy stage`,
	// possibly selective), this is skipped so the proposal is preserved.
	if err := autoStageIfIndexDescendant(repo, signCommit); err != nil {
		return err
	}

	// Step 3: Reconcile PolicyIndexRef vs PolicyStagingRef (fall back to
	// PolicyRef if no staged ref exists yet — bootstrap case where the
	// initial-policy mutations go directly to staging without an intermediate
	// stage call).
	stagingBase := PolicyStagingRef
	if _, err := repo.GetReference(PolicyStagingRef); err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return fmt.Errorf("failed to get policy staged reference: %w", err)
		}
		stagingBase = PolicyRef
	}

	return reconcileRefAgainstBase(repo, PolicyIndexRef, stagingBase, signCommit)
}

// autoStageIfIndexDescendant fast-forwards PolicyStagingRef to
// PolicyIndexRef when there's no pending proposal (PolicyStagingRef ==
// PolicyRef) and PolicyIndexRef is a strict descendant of PolicyStagingRef.
// This preserves the legacy "mutate → apply" workflow without bypassing
// explicit selective proposals (which produce a divergent PolicyStagingRef).
func autoStageIfIndexDescendant(repo *gitinterface.Repository, signCommit bool) error {
	policyTip, err := repo.GetReference(PolicyRef)
	if err != nil {
		if errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return nil
		}
		return err
	}

	stagedTip, err := repo.GetReference(PolicyStagingRef)
	if err != nil {
		if errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return nil
		}
		return err
	}

	// A pending proposal exists; respect it.
	if !stagedTip.Equal(policyTip) {
		return nil
	}

	stagingTip, err := repo.GetReference(PolicyIndexRef)
	if err != nil {
		if errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return nil
		}
		return err
	}

	if stagingTip.Equal(stagedTip) {
		return nil
	}

	isDescendant, err := repo.KnowsCommit(stagingTip, stagedTip)
	if err != nil {
		return err
	}
	if !isDescendant {
		// Diverged; defer to the rebase path in reconcileRefAgainstBase.
		return nil
	}

	if err := repo.SetReference(PolicyStagingRef, stagingTip); err != nil {
		return err
	}
	return rsl.NewReferenceEntry(PolicyStagingRef, stagingTip).Commit(repo, signCommit)
}

// reconcileRefAgainstBase reconciles targetRef against baseRef. The base must
// have a matching RSL entry (it's the authoritative anchor). The target may
// exist as a ref alone (e.g. PolicyIndexRef changes that haven't been
// recorded in the RSL yet); if it has an RSL entry, that entry must match the
// tip.
//
// If the target is ahead of base, nothing to do. If base is ahead, the target
// fast-forwards. If diverged, the target is rebuilt by overlaying
// targetState's metadata onto baseState's controller metadata.
//
// Returns nil if either ref is missing — nothing to reconcile.
func reconcileRefAgainstBase(repo *gitinterface.Repository, targetRef, baseRef string, signCommit bool) error {
	baseTip, baseEntry, baseFound, err := loadRefAndValidateRSLEntry(repo, baseRef)
	if err != nil {
		return err
	}
	if !baseFound {
		return nil
	}

	targetTip, err := repo.GetReference(targetRef)
	if err != nil {
		if errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return nil
		}
		return fmt.Errorf("failed to get reference %s: %w", targetRef, err)
	}

	// PolicyIndexRef is the local-only scratchpad — it has no RSL entry by
	// design. Every other ref must have a matching RSL entry if any exists.
	targetIsIndex := targetRef == PolicyIndexRef
	if !targetIsIndex {
		targetEntry, _, entryErr := rsl.GetLatestReferenceUpdaterEntry(repo, rsl.ForReference(targetRef))
		if entryErr != nil && !errors.Is(entryErr, rsl.ErrRSLEntryNotFound) {
			return fmt.Errorf("failed to get RSL entry for %s: %w", targetRef, entryErr)
		}
		if entryErr == nil && !targetEntry.GetTargetID().Equal(targetTip) {
			slog.Debug(fmt.Sprintf("%s tip does not match targetID in RSL entry, aborting.", targetRef))
			return ErrInvalidPolicy
		}
	}

	if baseTip.Equal(targetTip) {
		return nil
	}

	targetAhead, err := repo.KnowsCommit(targetTip, baseTip)
	if err != nil {
		return err
	}
	if targetAhead {
		// Target is properly ahead of base — done.
		return nil
	}

	baseAhead, err := repo.KnowsCommit(baseTip, targetTip)
	if err != nil {
		return err
	}
	if baseAhead {
		// Fast-forward target to base.
		if err := repo.SetReference(targetRef, baseTip); err != nil {
			return err
		}
		if targetIsIndex {
			// Local-only ref — no RSL entry.
			return nil
		}
		return rsl.NewReferenceEntry(targetRef, baseTip).Commit(repo, signCommit)
	}

	// Diverged — rebase target onto base.
	// This is safe because the unapplied changes are necessarily in
	// state.Metadata while propagations affect ControllerMetadata —
	// non-overlapping by construction.
	baseState, err := loadStateForEntry(repo, baseEntry)
	if err != nil {
		return err
	}

	// Load target state directly from the commit — the entry may be absent.
	targetState, err := loadStateFromCommit(repo, targetTip)
	if err != nil {
		return err
	}

	if err := repo.SetReference(targetRef, baseTip); err != nil {
		return err
	}
	if !targetIsIndex {
		if err := rsl.NewReferenceEntry(targetRef, baseTip).Commit(repo, signCommit); err != nil {
			return err
		}
	}

	// TODO: fix RSL entries for the target that are now orphaned

	newState := &State{
		Metadata:           targetState.Metadata,
		ControllerMetadata: baseState.ControllerMetadata,
		repository:         repo,
	}
	// PolicyIndexRef gets no RSL entry (local-only); other refs get one.
	_, err = newState.commitToRef(repo, targetRef, fmt.Sprintf("Rebase %s\n", targetRef), !targetIsIndex, signCommit)
	return err
}

// loadRefAndValidateRSLEntry returns the tip and latest RSL entry for refName
// after checking the invariant that the entry's TargetID equals the ref tip.
// Returns found=false when neither the ref nor an entry exist (valid
// not-yet-initialized state). Returns ErrInvalidPolicy when exactly one of
// them exists or when they don't match.
//
// PolicyIndexRef is special-cased: as a local-only scratchpad it is never
// recorded in the RSL, so its tip is returned with a nil entry whenever the
// ref exists.
func loadRefAndValidateRSLEntry(repo *gitinterface.Repository, refName string) (gitinterface.Hash, rsl.ReferenceUpdaterEntry, bool, error) {
	refExists := true
	tip, err := repo.GetReference(refName)
	if err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return nil, nil, false, fmt.Errorf("failed to get reference %s: %w", refName, err)
		}
		refExists = false
	}

	if refName == PolicyIndexRef {
		if !refExists {
			return nil, nil, false, nil
		}
		return tip, nil, true, nil
	}

	entryExists := true
	entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo, rsl.ForReference(refName))
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil, nil, false, fmt.Errorf("failed to get RSL entry for %s: %w", refName, err)
		}
		entryExists = false
	}

	switch {
	case refExists && entryExists:
		if !entry.GetTargetID().Equal(tip) {
			slog.Debug(fmt.Sprintf("%s tip does not match targetID in RSL entry, aborting.", refName))
			return nil, nil, false, ErrInvalidPolicy
		}
		return tip, entry, true, nil
	case (refExists && !entryExists) || (!refExists && entryExists):
		slog.Debug(fmt.Sprintf("Only one of %s ref and its RSL entry found, aborting.", refName))
		return nil, nil, false, ErrInvalidPolicy
	default:
		return nil, nil, false, nil
	}
}

func (s *State) GetRootKeys() ([]tuf.Principal, error) {
	rootMetadata, err := s.GetRootMetadata(false) // don't migrate: this may be for a write and we don't want to write tufv02 metadata yet
	if err != nil {
		return nil, err
	}

	return rootMetadata.GetRootPrincipals()
}

// GetRootMetadata returns the deserialized payload of the State's RootEnvelope.
// The `migrate` parameter determines if the schema must be converted to a newer
// version.
func (s *State) GetRootMetadata(migrate bool) (tuf.RootMetadata, error) {
	payloadBytes, err := s.Metadata.RootEnvelope.DecodeB64Payload()
	if err != nil {
		return nil, err
	}
	slog.Debug(fmt.Sprintf("Root metadata payload (%d bytes): %s", len(payloadBytes), string(payloadBytes)))
	return s.getRootMetadataFromBytes(payloadBytes, migrate)
}

func (s *State) GetControllerRootMetadata(controllerName string) (tuf.RootMetadata, error) {
	metadata, has := s.ControllerMetadata[controllerName]
	if !has {
		return nil, fmt.Errorf("%w: '%s'", ErrControllerMetadataNotFound, controllerName)
	}

	payloadBytes, err := metadata.RootEnvelope.DecodeB64Payload()
	if err != nil {
		return nil, err
	}
	return s.getRootMetadataFromBytes(payloadBytes, false) // never migrate
}

func (s *State) getRootMetadataFromBytes(metadataBytes []byte, migrate bool) (tuf.RootMetadata, error) {
	slog.Debug(fmt.Sprintf("getRootMetadataFromBytes: received %d bytes", len(metadataBytes)))
	if len(metadataBytes) > 0 {
		preview := metadataBytes
		const previewLimit = 512
		if len(preview) > previewLimit {
			preview = preview[:previewLimit]
		}
		slog.Debug(fmt.Sprintf("getRootMetadataFromBytes: content preview (first %d bytes): %s", len(preview), string(preview)))
	} else {
		slog.Debug("getRootMetadataFromBytes: content is EMPTY — this is why unmarshal will fail with 'unexpected end of JSON input'")
	}

	inspectRootMetadata := map[string]any{}
	if err := json.Unmarshal(metadataBytes, &inspectRootMetadata); err != nil {
		return nil, fmt.Errorf("unable to unmarshal root metadata (%d bytes): %w", len(metadataBytes), err)
	}

	schemaVersion, hasSchemaVersion := inspectRootMetadata["schemaVersion"]
	switch {
	case !hasSchemaVersion:
		// this is tufv01
		// Something that's not tufv01 may also lack the schemaVersion field and
		// enter this code path. At that point, we're relying on the unmarshal
		// to return something that's close to tufv01. We may see strange bugs
		// if this happens, but it's also likely someone trying to submit
		// incorrect metadata / trigger a version rollback, which we do want to
		// be aware of.
		rootMetadata := &tufv01.RootMetadata{}
		if err := json.Unmarshal(metadataBytes, rootMetadata); err != nil {
			return nil, fmt.Errorf("unable to unmarshal root metadata: %w", err)
		}

		if migrate {
			return migrations.MigrateRootMetadataV01ToV02(rootMetadata), nil
		}

		return rootMetadata, nil

	case schemaVersion == tufv02.RootVersion:
		rootMetadata := &tufv02.RootMetadata{}
		if err := json.Unmarshal(metadataBytes, rootMetadata); err != nil {
			return nil, fmt.Errorf("unable to unmarshal root metadata: %w", err)
		}

		return rootMetadata, nil

	default:
		return nil, tuf.ErrUnknownRootMetadataVersion
	}
}

// GetTargetsMetadata returns the deserialized payload of the State's
// TargetsEnvelope for the specified `roleName`.  The `migrate` parameter
// determines if the schema must be converted to a newer version.
func (s *State) GetTargetsMetadata(roleName string, migrate bool) (tuf.TargetsMetadata, error) {
	e := s.Metadata.TargetsEnvelope
	if roleName != TargetsRoleName {
		env, ok := s.Metadata.DelegationEnvelopes[roleName]
		if !ok {
			return nil, ErrMetadataNotFound
		}
		e = env
	}

	if e == nil {
		return nil, ErrMetadataNotFound
	}

	payloadBytes, err := e.DecodeB64Payload()
	if err != nil {
		return nil, err
	}

	inspectTargetsMetadata := map[string]any{}
	if err := json.Unmarshal(payloadBytes, &inspectTargetsMetadata); err != nil {
		return nil, fmt.Errorf("unable to unmarshal rule file metadata: %w", err)
	}

	schemaVersion, hasSchemaVersion := inspectTargetsMetadata["schemaVersion"]
	switch {
	case !hasSchemaVersion:
		// this is tufv01
		// Something that's not tufv01 may also lack the schemaVersion field and
		// enter this code path. At that point, we're relying on the unmarshal
		// to return something that's close to tufv01. We may see strange bugs
		// if this happens, but it's also likely someone trying to submit
		// incorrect metadata / trigger a version rollback, which we do want to
		// be aware of.
		targetsMetadata := &tufv01.TargetsMetadata{}
		if err := json.Unmarshal(payloadBytes, targetsMetadata); err != nil {
			return nil, fmt.Errorf("unable to unmarshal rule file metadata: %w", err)
		}

		if migrate {
			return migrations.MigrateTargetsMetadataV01ToV02(targetsMetadata), nil
		}

		return targetsMetadata, nil

	case schemaVersion == tufv02.TargetsVersion:
		targetsMetadata := &tufv02.TargetsMetadata{}
		if err := json.Unmarshal(payloadBytes, targetsMetadata); err != nil {
			return nil, fmt.Errorf("unable to unmarshal rule file metadata: %w", err)
		}

		return targetsMetadata, nil

	default:
		return nil, tuf.ErrUnknownTargetsMetadataVersion
	}
}

func (s *State) HasTargetsRole(roleName string) bool {
	if roleName == TargetsRoleName {
		return s.Metadata.TargetsEnvelope != nil
	}

	_, ok := s.Metadata.DelegationEnvelopes[roleName]
	return ok
}

func (s *State) HasRuleName(name string) bool {
	return s.ruleNames.Has(name)
}

// preprocess handles several "one time" tasks when the state is first loaded.
// This includes things like loading the set of rule names present in the state,
// checking if it has file rules, etc.
func (s *State) preprocess() error {
	rootMetadata, err := s.GetRootMetadata(false)
	if err != nil {
		slog.Debug("Encountered error while getting root metadata during state preprocessing!")
		return err
	}

	s.Hooks = make(map[tuf.HookStage][]tuf.Hook, 2)

	hooks, err := rootMetadata.GetHooks(tuf.HookStagePreCommit)
	if err != nil {
		slog.Debug("Encountered error while getting pre-commit hooks!")
		if !errors.Is(err, tuf.ErrNoHooksDefined) {
			return err
		}
	}

	if s.Hooks[tuf.HookStagePreCommit] == nil {
		s.Hooks[tuf.HookStagePreCommit] = []tuf.Hook{}
	}

	s.Hooks[tuf.HookStagePreCommit] = append(s.Hooks[tuf.HookStagePreCommit], hooks...)

	hooks, err = rootMetadata.GetHooks(tuf.HookStagePrePush)
	if err != nil {
		slog.Debug("Encountered error while getting pre-push hooks!")
		if !errors.Is(err, tuf.ErrNoHooksDefined) {
			return err
		}
	}

	if s.Hooks[tuf.HookStagePrePush] == nil {
		s.Hooks[tuf.HookStagePrePush] = []tuf.Hook{}
	}

	s.Hooks[tuf.HookStagePrePush] = append(s.Hooks[tuf.HookStagePrePush], hooks...)

	globalRules := rootMetadata.GetGlobalRules()
	if len(globalRules) > 0 {
		s.globalRules = map[string][]tuf.GlobalRule{
			"": rootMetadata.GetGlobalRules(),
		}
	}

	if s.allPrincipals == nil {
		s.allPrincipals = map[string]tuf.Principal{}
	}

	for principalID, principal := range rootMetadata.GetPrincipals() {
		s.allPrincipals[principalID] = principal
	}

	s.GitHubApps, err = rootMetadata.GetGitHubAppEntries()
	if err != nil {
		slog.Debug("Encountered error while getting GitHub App entries!")
		return err
	}

	if s.Metadata.TargetsEnvelope == nil {
		return nil
	}

	s.ruleNames = set.NewSet[string]()

	targetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName, false)

	if err != nil {
		return err
	}

	for principalID, principal := range targetsMetadata.GetPrincipals() {
		s.allPrincipals[principalID] = principal
	}

	for _, rule := range targetsMetadata.GetRules() {
		if rule.ID() == tuf.AllowRuleName {
			continue
		}

		if s.ruleNames.Has(rule.ID()) {
			return tuf.ErrDuplicatedRuleName
		}

		s.ruleNames.Add(rule.ID())

		if !s.hasFileRule {
			patterns := rule.GetProtectedNamespaces()
			for _, pattern := range patterns {
				if strings.HasPrefix(pattern, fileRuleScheme) {
					s.hasFileRule = true
					break
				}
			}
		}
	}

	if len(s.Metadata.DelegationEnvelopes) != 0 {
		for delegatedRoleName := range s.Metadata.DelegationEnvelopes {
			delegatedMetadata, err := s.GetTargetsMetadata(delegatedRoleName, false)
			if err != nil {
				return err
			}

			for principalID, principal := range delegatedMetadata.GetPrincipals() {
				s.allPrincipals[principalID] = principal
			}

			for _, rule := range delegatedMetadata.GetRules() {
				if rule.ID() == tuf.AllowRuleName {
					continue
				}

				if s.ruleNames.Has(rule.ID()) {
					return tuf.ErrDuplicatedRuleName
				}

				s.ruleNames.Add(rule.ID())

				if !s.hasFileRule {
					patterns := rule.GetProtectedNamespaces()
					for _, pattern := range patterns {
						if strings.HasPrefix(pattern, fileRuleScheme) {
							s.hasFileRule = true
							break
						}
					}
				}
			}
		}
	}

	for controllerName := range s.ControllerMetadata {
		controllerRootMetadata, err := s.GetControllerRootMetadata(controllerName)
		if err != nil {
			return err
		}

		globalRules := controllerRootMetadata.GetGlobalRules()
		if len(globalRules) > 0 {
			if s.globalRules == nil {
				s.globalRules = map[string][]tuf.GlobalRule{}
			}

			s.globalRules[controllerName] = globalRules
		}
	}

	return nil
}

func (s *State) getRootVerifier() (*SignatureVerifier, error) {
	rootMetadata, err := s.GetRootMetadata(false)
	if err != nil {
		return nil, err
	}

	principals, err := rootMetadata.GetRootPrincipals()
	if err != nil {
		return nil, err
	}

	threshold, err := rootMetadata.GetRootThreshold()
	if err != nil {
		return nil, err
	}

	return &SignatureVerifier{
		repository: s.repository,
		principals: principals,
		threshold:  threshold,
	}, nil
}

func (s *State) getTargetsVerifier() (*SignatureVerifier, error) {
	rootMetadata, err := s.GetRootMetadata(false)
	if err != nil {
		return nil, err
	}

	principals, err := rootMetadata.GetPrimaryRuleFilePrincipals()
	if err != nil {
		return nil, err
	}

	threshold, err := rootMetadata.GetPrimaryRuleFileThreshold()
	if err != nil {
		return nil, err
	}

	return &SignatureVerifier{
		repository: s.repository,
		principals: principals,
		threshold:  threshold,
	}, nil
}

// loadStateForEntry returns the State for a specified RSL reference entry for
// the policy namespace. This helper is focused on reading the Git object store
// and loading the policy contents. Typically, LoadCurrentState of LoadState
// must be used. The exception is VerifyRelative... which performs root
// verification between consecutive policy states.
func loadStateForEntry(repo *gitinterface.Repository, entry rsl.ReferenceUpdaterEntry) (*State, error) {
	if entry.GetRefName() != PolicyRef && entry.GetRefName() != PolicyIndexRef && entry.GetRefName() != PolicyStagingRef {
		return nil, rsl.ErrRSLEntryDoesNotMatchRef
	}

	state, err := loadStateFromCommit(repo, entry.GetTargetID())
	if err != nil {
		return nil, err
	}
	state.loadedEntry = entry
	return state, nil
}

func loadStateFromCommit(repo *gitinterface.Repository, commitID gitinterface.Hash) (*State, error) {
	commitTreeID, err := repo.GetCommitTreeID(commitID)
	if err != nil {
		return nil, err
	}

	treeItems, err := repo.GetTreeItems(commitTreeID)
	if err != nil {
		return nil, err
	}

	// metadataQueue is populated with metadata/ subtrees we want to load for
	// either the current repository or its controllers.
	metadataQueue := []*policyTreeItem{{name: "", treeID: treeItems[metadataTreeEntryName]}}
	if controllerTreeID, hasControllers := treeItems[tuf.GittufControllerPrefix]; hasControllers {
		controllerEntries, err := repo.GetTreeItems(controllerTreeID)
		if err != nil {
			return nil, err
		}

		for controllerName, treeID := range controllerEntries {
			metadataQueue = append(metadataQueue, &policyTreeItem{name: controllerName, treeID: treeID})
		}
	}

	state := &State{repository: repo}

	for len(metadataQueue) != 0 {
		currentMetadataEntry := metadataQueue[0]
		metadataQueue = metadataQueue[1:]

		slog.Debug(fmt.Sprintf("Loading policy for '%s' from '%s'...", currentMetadataEntry.name, currentMetadataEntry.treeID.String()))

		metadataItems, err := repo.GetTreeItems(currentMetadataEntry.treeID)
		if err != nil {
			return nil, err
		}

		stateMetadata := &StateMetadata{}
		for name, blobID := range metadataItems {
			contents, err := repo.ReadBlob(blobID)
			if err != nil {
				return nil, err
			}

			env := &sslibdsse.Envelope{}
			if err := json.Unmarshal(contents, env); err != nil {
				return nil, err
			}

			switch name {
			case fmt.Sprintf("%s.json", RootRoleName):
				stateMetadata.RootEnvelope = env

			case fmt.Sprintf("%s.json", TargetsRoleName):
				stateMetadata.TargetsEnvelope = env

			default:
				if stateMetadata.DelegationEnvelopes == nil {
					stateMetadata.DelegationEnvelopes = map[string]*sslibdsse.Envelope{}
				}

				stateMetadata.DelegationEnvelopes[strings.TrimSuffix(name, ".json")] = env
			}
		}

		if currentMetadataEntry.name == "" {
			state.Metadata = stateMetadata
		} else {
			if state.ControllerMetadata == nil {
				state.ControllerMetadata = map[string]*StateMetadata{}
			}

			state.ControllerMetadata[currentMetadataEntry.name] = stateMetadata
		}
	}

	slog.Debug("Loaded current repository policy!")

	for name := range state.ControllerMetadata {
		slog.Debug(fmt.Sprintf("Loaded policy from controller '%s'!", name))
	}

	if err := state.preprocess(); err != nil {
		return nil, err
	}

	return state, nil
}

type policyTreeItem struct {
	name   string
	treeID gitinterface.Hash
}

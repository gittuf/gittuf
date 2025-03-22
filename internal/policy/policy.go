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
	"github.com/gittuf/gittuf/internal/gitinterface"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/internal/tuf/migrations"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
)

const (
	// PolicyRef defines the Git namespace used for gittuf policies.
	PolicyRef = "refs/gittuf/policy"

	// PolicyStagingRef defines the Git namespace used as a staging area when creating or updating gittuf policies.
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
	ErrControllerMetadataNotVerified = errors.New("unable to verify controller repository metadata")
)

// State contains the full set of metadata and root keys present in a policy
// state.
type State struct {
	Metadata           *StateMetadata
	ControllerMetadata map[string]*StateMetadata

	Hooks map[tuf.HookStage][]tuf.Hook

	githubAppApprovalsTrusted bool
	githubAppKeys             []tuf.Principal
	githubAppRoleName         string

	repository     *gitinterface.Repository
	loadedEntry    rsl.ReferenceUpdaterEntry
	verifiersCache map[string][]*SignatureVerifier
	ruleNames      *set.Set[string]
	allPrincipals  map[string]tuf.Principal
	hasFileRule    bool
	globalRules    []tuf.GlobalRule
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

	verifiers, err := s.findVerifiersForPathIfProtected(path)
	if err != nil {
		return nil, err
	}

	if len(verifiers) > 0 {
		// protected, we have specific set of verifiers to return
		slog.Debug(fmt.Sprintf("Path '%s' is explicitly protected, returning corresponding verifiers...", path))
		// add to cache
		s.verifiersCache[path] = verifiers
		// return verifiers
		return verifiers, nil
	}

	slog.Debug("Checking if any global constraints exist")
	if len(s.globalRules) == 0 {
		slog.Debug("No global constraints found")
		s.verifiersCache[path] = verifiers
		return verifiers, nil
	}

	slog.Debug("Global constraints found, returning exhaustive verifier...")
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

	verifiers = []*SignatureVerifier{verifier}

	// Note: we could loop through all global constraints and create a
	// verifier with all principals but targeting a specific constraint (or
	// an aggregate constraint that has the highest threshold requirement of
	// all the constraints that match path). However, this probably paints
	// us into a corner (only threshold requirements between two constraints
	// can be compared, we may have uncomparable constraints later), and we
	// would also want to verify every applicable global constraint for
	// safety, so we would be doing extra work for no reason.

	// add to cache
	s.verifiersCache[path] = verifiers
	// return verifiers
	return verifiers, nil
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

		for {
			if len(currentDelegationGroup) <= 1 {
				// Only allow rule found in the current group
				break
			}

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
	if rootMetadata.IsGitHubAppApprovalTrusted() {
		// Check that the GitHub app role is declared
		_, err := rootMetadata.GetGitHubAppPrincipals()
		if err != nil {
			return err
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

		targetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName, false) // don't migrate: this may be for a write and we don't want to write tufv02 metadata yet
		if err != nil {
			return err
		}

		// Check reachable delegations
		reachedDelegations := map[string]bool{}
		for delegatedRoleName := range s.Metadata.DelegationEnvelopes {
			reachedDelegations[delegatedRoleName] = false
		}

		delegationsQueue := targetsMetadata.GetRules()
		delegationKeys := targetsMetadata.GetPrincipals()
		for {
			// The last entry in the queue is always the allow rule, which we don't
			// process during DFS
			if len(delegationsQueue) <= 1 {
				break
			}

			delegation := delegationsQueue[0]
			delegationsQueue = delegationsQueue[1:]

			if s.HasTargetsRole(delegation.ID()) {
				reachedDelegations[delegation.ID()] = true

				env := s.Metadata.DelegationEnvelopes[delegation.ID()]

				principals := []tuf.Principal{}
				for _, principalID := range delegation.GetPrincipalIDs().Contents() {
					principals = append(principals, delegationKeys[principalID])
				}

				verifier := &SignatureVerifier{
					repository: s.repository,
					name:       delegation.ID(),
					principals: principals,
					threshold:  delegation.GetThreshold(),
				}

				if _, err := verifier.Verify(ctx, gitinterface.ZeroHash, env); err != nil {
					return err
				}

				delegatedMetadata, err := s.GetTargetsMetadata(delegation.ID(), false) // don't migrate: this may be for a write and we don't want to write tufv02 metadata yet
				if err != nil {
					return err
				}

				delegationsQueue = append(delegatedMetadata.GetRules(), delegationsQueue...)
				for keyID, key := range delegatedMetadata.GetPrincipals() {
					delegationKeys[keyID] = key
				}
			}
		}

		for _, reached := range reachedDelegations {
			if !reached {
				return ErrDanglingDelegationMetadata
			}
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

// Commit verifies and writes the State to the policy-staging namespace.
func (s *State) Commit(repo *gitinterface.Repository, commitMessage string, createRSLEntry, signCommit bool) error {
	if len(commitMessage) == 0 {
		commitMessage = DefaultCommitMessage
	}

	// Get treeIDs for state.Metadata and each of the state.ControllerMetadata entries
	allTreeEntries := []gitinterface.TreeEntry{}

	stateMetadataTreeID, err := s.Metadata.WriteTree(repo)
	if err != nil {
		return nil
	}
	allTreeEntries = append(allTreeEntries, gitinterface.NewEntryTree(metadataTreeEntryName, stateMetadataTreeID))

	for absoluteControllerPath, metadata := range s.ControllerMetadata {
		// If path is "1", it should become gittuf-controller/1/metadata
		// If path is "1/2", it should become gittuf-controller/1/gittuf-controller/2/metadata
		// If path is "1/2/3", it should become gittuf-controller/1/gittuf-controller/2/gittuf-controller/3/metadata

		pathComponents := strings.Split(absoluteControllerPath, "/")
		newPath := strings.Join(pathComponents, "/"+tuf.GittufControllerPrefix+"/")
		// For "1/2/3", newPath is now "1/gittuf-controller/2/gittuf-controller/3"
		// Add the controller prefix and metadata tree suffix
		newPath = fmt.Sprintf("%s/%s/%s", tuf.GittufControllerPrefix, newPath, metadataTreeEntryName)

		stateMetadataTreeID, err := metadata.WriteTree(repo)
		if err != nil {
			return nil
		}
		allTreeEntries = append(allTreeEntries, gitinterface.NewEntryBlob(newPath, stateMetadataTreeID))
	}

	for stage, hookSet := range s.Hooks {
		for _, hook := range hookSet {
			hookPath := fmt.Sprintf("%s/%s/%s", tuf.HooksPrefix, stage.String(), hook.ID())
			allTreeEntries = append(allTreeEntries, gitinterface.NewEntryBlob(hookPath, hook.GetBlobID()))
		}
	}

	treeBuilder := gitinterface.NewTreeBuilder(repo)
	policyRootTreeID, err := treeBuilder.WriteTreeFromEntries(allTreeEntries)
	if err != nil {
		return err
	}

	originalCommitID, err := repo.GetReference(PolicyStagingRef)
	if err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return err
		}
	}

	commitID, err := repo.Commit(policyRootTreeID, PolicyStagingRef, commitMessage, signCommit)
	if err != nil {
		return err
	}

	// We must reset to original policy commit if err != nil from here onwards.
	if createRSLEntry {
		if err := rsl.NewReferenceEntry(PolicyStagingRef, commitID).Commit(repo, signCommit); err != nil {
			if !originalCommitID.IsZero() {
				return repo.ResetDueToError(err, PolicyStagingRef, originalCommitID)
			}

			return err
		}
	}

	return nil
}

// Apply takes valid changes from the policy staging ref, and fast-forward
// merges it into the policy ref. Apply only takes place if the latest state on
// the policy staging ref is valid. This prevents invalid changes to the policy
// taking affect, and allowing new changes, that until signed by multiple users
// would be invalid to be made, by utilizing the policy staging ref.
func Apply(ctx context.Context, repo *gitinterface.Repository, signRSLEntry bool) error {
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

	// Get the reference for the PolicyStagingRef
	policyStagingTip, err := repo.GetReference(PolicyStagingRef)
	if err != nil {
		return fmt.Errorf("failed to get policy staging reference %s: %w", PolicyStagingRef, err)
	}

	// Check if the PolicyStagingRef is ahead of PolicyRef (fast-forward)

	if !policyTip.IsZero() {
		// This check ensures that the policy staging branch is a direct forward progression of the policy branch,
		// preventing any overwrites of policy history and maintaining a linear policy evolution, since a
		// fast-forward merge does not work with a non-linear history.

		// This is only being checked if there are no problems finding the tip of the policy ref, since if there
		// is no tip, then it cannot be an ancestor of the tip of the policy staging ref
		isAncestor, err := repo.KnowsCommit(policyStagingTip, policyTip)
		if err != nil {
			return fmt.Errorf("failed to check if policy commit is ancestor of policy staging commit: %w", err)
		}
		if !isAncestor {
			return ErrNotAncestor
		}
	}

	// using LoadCurrentState to load and verify if the PolicyStagingRef's
	// latest state is valid
	state, err := LoadCurrentState(ctx, repo, PolicyStagingRef)
	if err != nil {
		return fmt.Errorf("failed to load current state: %w", err)
	}
	if err := state.Verify(ctx); err != nil {
		return fmt.Errorf("staged policy is invalid: %w", err)
	}

	// Update the reference for the base to point to the new commit
	if err := repo.SetReference(PolicyRef, policyStagingTip); err != nil {
		return fmt.Errorf("failed to set new policy reference: %w", err)
	}

	if err := rsl.NewReferenceEntry(PolicyRef, policyStagingTip).Commit(repo, signRSLEntry); err != nil {
		if !policyTip.IsZero() {
			return repo.ResetDueToError(err, PolicyRef, policyTip)
		}

		return err
	}

	return nil
}

// Discard resets the policy staging ref, discarding any changes made to the policy staging ref.
func Discard(repo *gitinterface.Repository) error {
	policyTip, err := repo.GetReference(PolicyRef)
	if err != nil {
		if errors.Is(err, gitinterface.ErrReferenceNotFound) {
			if err := repo.DeleteReference(PolicyStagingRef); err != nil && !errors.Is(err, gitinterface.ErrReferenceNotFound) {
				return fmt.Errorf("failed to delete policy staging reference %s: %w", PolicyStagingRef, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get policy reference %s: %w", PolicyRef, err)
	}

	// Reset PolicyStagingRef to match the actual policy ref
	if err := repo.SetReference(PolicyStagingRef, policyTip); err != nil {
		return fmt.Errorf("failed to reset policy staging reference %s: %w", PolicyStagingRef, err)
	}

	return nil
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

	inspectRootMetadata := map[string]any{}
	if err := json.Unmarshal(payloadBytes, &inspectRootMetadata); err != nil {
		return nil, fmt.Errorf("unable to unmarshal root metadata: %w", err)
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
		if err := json.Unmarshal(payloadBytes, rootMetadata); err != nil {
			return nil, fmt.Errorf("unable to unmarshal root metadata: %w", err)
		}

		if migrate {
			return migrations.MigrateRootMetadataV01ToV02(rootMetadata), nil
		}

		return rootMetadata, nil

	case schemaVersion == tufv02.RootVersion:
		rootMetadata := &tufv02.RootMetadata{}
		if err := json.Unmarshal(payloadBytes, rootMetadata); err != nil {
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
		return err
	}

	s.Hooks = make(map[tuf.HookStage][]tuf.Hook, 2)

	hooks, err := rootMetadata.GetHooks(tuf.HookStagePreCommit)
	if err != nil {
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
		if !errors.Is(err, tuf.ErrNoHooksDefined) {
			return err
		}
	}

	if s.Hooks[tuf.HookStagePrePush] == nil {
		s.Hooks[tuf.HookStagePrePush] = []tuf.Hook{}
	}

	s.Hooks[tuf.HookStagePrePush] = append(s.Hooks[tuf.HookStagePrePush], hooks...)

	s.globalRules = rootMetadata.GetGlobalRules()

	if s.allPrincipals == nil {
		s.allPrincipals = map[string]tuf.Principal{}
	}

	for principalID, principal := range rootMetadata.GetPrincipals() {
		s.allPrincipals[principalID] = principal
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

	if len(s.Metadata.DelegationEnvelopes) == 0 {
		return nil
	}

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
	if entry.GetRefName() != PolicyRef && entry.GetRefName() != PolicyStagingRef {
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
	// controllerQueue is populated with gittuf-controller/ subtrees as we need
	// to unwrap them to identify the applicable metadata/ tree entries.
	// Here, the `name` parameter identifies the set of parents. If the
	// controller subtree is directly declared in the current repository, then
	// its name is empty.
	controllerQueue := []*policyTreeItem{}
	if controllerTreeID, hasController := treeItems[tuf.GittufControllerPrefix]; hasController {
		controllerQueue = append(controllerQueue, &policyTreeItem{name: "", treeID: controllerTreeID})
	}

	for len(controllerQueue) != 0 {
		currentControllerEntry := controllerQueue[0]
		controllerQueue = controllerQueue[1:]

		controllerTreeItems, err := repo.GetTreeItems(currentControllerEntry.treeID)
		if err != nil {
			return nil, err
		}

		// controllerTreeItems should be 1+ subtrees that have the name of the controller in question.
		// Each subtree in turn has a metadata subtree and optionally more controller subtrees.

		for controllerName, subtreeID := range controllerTreeItems {
			subtreeItems, err := repo.GetTreeItems(subtreeID)
			if err != nil {
				return nil, err
			}

			absoluteControllerName := currentControllerEntry.name + "/" + controllerName
			absoluteControllerName = strings.Trim(absoluteControllerName, "/")

			for treeName, treeID := range subtreeItems {
				if treeName == metadataTreeEntryName {
					metadataQueue = append(metadataQueue, &policyTreeItem{name: absoluteControllerName, treeID: treeID})
				} else if treeName == tuf.GittufControllerPrefix {
					controllerQueue = append(controllerQueue, &policyTreeItem{name: absoluteControllerName, treeID: treeID})
				}
			}
		}
	}

	state := &State{repository: repo}

	for len(metadataQueue) != 0 {
		currentMetadataEntry := metadataQueue[0]
		metadataQueue = metadataQueue[1:]

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

	if err := state.preprocess(); err != nil {
		return nil, err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return nil, err
	}

	state.githubAppApprovalsTrusted = rootMetadata.IsGitHubAppApprovalTrusted()

	githubAppPrincipals, err := rootMetadata.GetGitHubAppPrincipals()
	if err == nil {
		state.githubAppKeys = githubAppPrincipals
		state.githubAppRoleName = tuf.GitHubAppRoleName
	} else if state.githubAppApprovalsTrusted {
		return nil, tuf.ErrGitHubAppInformationNotFoundInRoot
	}

	return state, nil
}

type policyTreeItem struct {
	name   string
	treeID gitinterface.Hash
}

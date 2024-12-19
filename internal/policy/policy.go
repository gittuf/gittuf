// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
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

	hooksTreeEntryName = "hooks"

	gitReferenceRuleScheme = "git"
	fileRuleScheme         = "file"
)

var (
	ErrMetadataNotFound           = errors.New("unable to find requested metadata file; has it been initialized?")
	ErrDanglingDelegationMetadata = errors.New("unreachable targets metadata found")
	ErrPolicyNotFound             = errors.New("cannot find policy")
	ErrUnableToMatchRootKeys      = errors.New("unable to match root public keys, gittuf policy is in a broken state")
	ErrNotAncestor                = errors.New("cannot apply changes since policy is not an ancestor of the policy staging")
)

// State contains the full set of metadata and root keys present in a policy
// state.
type State struct {
	RootEnvelope        *sslibdsse.Envelope
	TargetsEnvelope     *sslibdsse.Envelope
	DelegationEnvelopes map[string]*sslibdsse.Envelope
	RootPublicKeys      []tuf.Principal

	githubAppApprovalsTrusted bool
	githubAppKeys             []tuf.Principal

	repository     *gitinterface.Repository
	verifiersCache map[string][]*Verifier
	ruleNames      *set.Set[string]
	hasFileRule    bool
}

// LoadState returns the State of the repository's policy corresponding to the
// entry. It verifies the root of trust for the state from the initial policy
// entry in the RSL. If no policy states are found and the entry is for the
// policy-staging ref, that entry is returned with no verification.
func LoadState(ctx context.Context, repo *gitinterface.Repository, requestedEntry *rsl.ReferenceEntry) (*State, error) {
	// Regardless of whether we've been asked for policy ref or staging ref,
	// we want to examine and verify consecutive policy states that appear
	// before the entry. This is why we don't just load the state and return
	// if entry is for the staging ref.

	searcher := newSearcher(repo)

	firstPolicyEntry, err := searcher.FindFirstPolicyEntry()
	if err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			// we don't have a policy entry yet
			// we just return the state for the requested entry
			return loadStateForEntry(repo, requestedEntry)
		}
		return nil, err
	}

	// check if firstPolicyEntry is **after** requested entry
	// this can happen when the requested entry is for policy-staging before
	// Apply() was ever called
	knows, err := repo.KnowsCommit(firstPolicyEntry.ID, requestedEntry.ID)
	if err != nil {
		return nil, err
	}
	if knows {
		// the first policy entry knows the requested entry, meaning the
		// requested entry is an ancestor of the first policy entry
		// we just return the state for the requested entry
		return loadStateForEntry(repo, requestedEntry)
	}

	// If requestedEntry.RefName == policy, then allPolicyEntries includes requestedEntry
	// If requestedEntry.RefName == policy-staging, then allPolicyEntries does not include requestedEntry
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

	slog.Debug(fmt.Sprintf("Trusting root of trust for initial policy '%s'...", firstPolicyEntry.ID.String()))
	verifiedState := initialPolicyState
	for _, entry := range allPolicyEntries[1:] {
		if entry.RefName != PolicyRef {
			// The searcher _may_ include refs/gittuf/attestations
			// etc. which should be skipped
			continue
		}

		underTestState, err := loadStateForEntry(repo, entry)
		if err != nil {
			return nil, err
		}

		slog.Debug(fmt.Sprintf("Verifying root of trust for policy '%s'...", entry.ID.String()))
		if err := verifiedState.VerifyNewState(ctx, underTestState); err != nil {
			return nil, fmt.Errorf("unable to verify roots of trust for policy states: %w", err)
		}

		verifiedState = underTestState
	}

	if requestedEntry.RefName == PolicyRef {
		// We've already loaded it and done successive verification as
		// it was included in allPolicyEntries
		// This state is stored in verifiedState, we can do an internal
		// verification check and return

		if err := verifiedState.Verify(ctx); err != nil {
			return nil, fmt.Errorf("requested state has invalidly signed metadata: %w", err)
		}

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
func LoadCurrentState(ctx context.Context, repo *gitinterface.Repository, ref string) (*State, error) {
	entry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(ref))
	if err != nil {
		return nil, err
	}

	return LoadState(ctx, repo, entry)
}

// LoadFirstState returns the State corresponding to the repository's first
// active policy. It does not verify the root of trust since it is the initial policy.
func LoadFirstState(ctx context.Context, repo *gitinterface.Repository) (*State, error) {
	firstEntry, _, err := rsl.GetFirstReferenceEntryForRef(repo, PolicyRef)
	if err != nil {
		return nil, err
	}

	return LoadState(ctx, repo, firstEntry)
}

// GetStateForCommit scans the RSL to identify the first time a commit was seen
// in the repository. The policy preceding that RSL entry is returned as the
// State to be used for verifying the commit's signature. If the commit hasn't
// been seen in the repository previously, no policy state is returned. Also, no
// error is returned. Identifying the policy in this case is left to the calling
// workflow.
func GetStateForCommit(ctx context.Context, repo *gitinterface.Repository, commitID gitinterface.Hash) (*State, error) {
	firstSeenEntry, _, err := rsl.GetFirstReferenceEntryForCommit(repo, commitID)
	if err != nil {
		if errors.Is(err, rsl.ErrNoRecordOfCommit) {
			return nil, nil
		}
		return nil, err
	}

	commitPolicyEntry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(PolicyRef), rsl.BeforeEntryID(firstSeenEntry.ID))
	if err != nil {
		return nil, err
	}

	return LoadState(ctx, repo, commitPolicyEntry)
}

// FindVerifiersForPath identifies the trusted set of verifiers for the
// specified path. While walking the delegation graph for the path, signatures
// for delegated metadata files are verified using the verifier context.
func (s *State) FindVerifiersForPath(path string) ([]*Verifier, error) {
	if s.verifiersCache == nil {
		slog.Debug("Initializing path cache in policy...")
		s.verifiersCache = map[string][]*Verifier{}
	} else if verifiers, cacheHit := s.verifiersCache[path]; cacheHit {
		// Cache hit for this path in this policy
		slog.Debug(fmt.Sprintf("Found cached verifiers for path '%s'", path))
		return verifiers, nil
	}

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
	verifiers := []*Verifier{}
	for {
		if len(groupedDelegations) == 0 {
			s.verifiersCache[path] = verifiers
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
				verifier := &Verifier{
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

// Verify verifies the contents of the State for internal consistency.
// Specifically, it checks that the root keys in the root role match the ones
// stored on disk in the state. Further, it also verifies the signatures of the
// top level Targets role and all reachable delegated Targets roles. Any
// unreachable role returns an error.
func (s *State) Verify(ctx context.Context) error {
	// Check consistency of root keys
	rootKeys, err := s.GetRootKeys()
	if err != nil {
		return err
	}
	// TODO: do we need this?
	if !verifyRootKeysMatch(rootKeys, s.RootPublicKeys) {
		return ErrUnableToMatchRootKeys
	}

	rootVerifier, err := s.getRootVerifier()
	if err != nil {
		return err
	}

	if _, err := rootVerifier.Verify(ctx, gitinterface.ZeroHash, s.RootEnvelope); err != nil {
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

	// Check top-level targets
	if s.TargetsEnvelope == nil {
		return nil
	}

	targetsVerifier, err := s.getTargetsVerifier()
	if err != nil {
		return err
	}

	if _, err := targetsVerifier.Verify(ctx, gitinterface.ZeroHash, s.TargetsEnvelope); err != nil {
		return err
	}

	targetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName, false) // don't migrate: this may be for a write and we don't want to write tufv02 metadata yet
	if err != nil {
		return err
	}

	// Check reachable delegations
	reachedDelegations := map[string]bool{}
	for delegatedRoleName := range s.DelegationEnvelopes {
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

			env := s.DelegationEnvelopes[delegation.ID()]

			principals := []tuf.Principal{}
			for _, principalID := range delegation.GetPrincipalIDs().Contents() {
				principals = append(principals, delegationKeys[principalID])
			}

			verifier := &Verifier{
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

	return nil
}

// Commit verifies and writes the State to the policy-staging namespace. It also creates
// an RSL entry recording the new tip of the policy-staging namespace.
func (s *State) Commit(repo *gitinterface.Repository, commitMessage string, signCommit bool) error {
	if len(commitMessage) == 0 {
		commitMessage = DefaultCommitMessage
	}

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

	allTreeEntries := map[string]gitinterface.Hash{}

	for name, env := range metadata {
		envContents, err := json.Marshal(env)
		if err != nil {
			return err
		}

		blobID, err := repo.WriteBlob(envContents)
		if err != nil {
			return err
		}

		allTreeEntries[path.Join(metadataTreeEntryName, name+".json")] = blobID
	}

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	policyRootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(allTreeEntries)
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

	if err := rsl.NewReferenceEntry(PolicyStagingRef, commitID).Commit(repo, signCommit); err != nil {
		if !originalCommitID.IsZero() {
			return repo.ResetDueToError(err, PolicyStagingRef, originalCommitID)
		}

		return err
	}

	return nil
}

func (s *State) CommitHooks(repo *gitinterface.Repository, commitMessage string, blobsMapping map[string]gitinterface.Hash, sign bool) error {
	if len(commitMessage) == 0 {
		commitMessage = DefaultCommitMessage
	}

	metadata := map[string]*sslibdsse.Envelope{}
	if s.TargetsEnvelope != nil {
		metadata[TargetsRoleName] = s.TargetsEnvelope
	}
	if s.RootEnvelope != nil {
		metadata[RootRoleName] = s.RootEnvelope
	}

	allTreeEntries := map[string]gitinterface.Hash{}
	for name, env := range metadata {
		envContents, err := json.Marshal(env)
		if err != nil {
			return err
		}

		blobID, err := repo.WriteBlob(envContents)
		if err != nil {
			return err
		}

		allTreeEntries[path.Join(metadataTreeEntryName, name+".json")] = blobID
	}

	// structure of blobsMapping is {hookname: blobID}
	if len(blobsMapping) > 0 {
		for hookName, blobID := range blobsMapping {
			allTreeEntries[path.Join(hooksTreeEntryName, hookName)] = blobID
		}
	}

	slog.Debug("building and populating new tree...")
	treeBuilder := gitinterface.NewTreeBuilder(repo)

	hooksRootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(allTreeEntries)
	if err != nil {
		return err
	}

	originalCommitID, err := repo.GetReference(PolicyStagingRef)
	if err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return err
		}
	}
	commitID, err := repo.Commit(hooksRootTreeID, PolicyStagingRef, commitMessage, sign)
	if err != nil {
		return err
	}
	slog.Debug("committing hooks metadata successful!")

	// record changes to RSL; reset to original policy commit if err != nil

	newReferenceEntry := rsl.NewReferenceEntry(PolicyStagingRef, commitID)
	if err := newReferenceEntry.Commit(repo, true); err != nil {
		if !originalCommitID.IsZero() {
			return repo.ResetDueToError(err, PolicyStagingRef, originalCommitID)
		}

		return err
	}
	slog.Debug("RSL entry recording successful!")
	hooksTip, err := repo.GetReference(PolicyStagingRef)
	if err != nil {
		return err
	}

	if err := repo.SetReference(PolicyStagingRef, hooksTip); err != nil {
		return fmt.Errorf("failed to set new hooks reference: %w", err)
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
	policyTip, err := repo.GetReference(PolicyRef)
	if err != nil {
		if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return fmt.Errorf("failed to get policy reference %s: %w", PolicyRef, err)
		}
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
	payloadBytes, err := s.RootEnvelope.DecodeB64Payload()
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
	e := s.TargetsEnvelope
	if roleName != TargetsRoleName {
		env, ok := s.DelegationEnvelopes[roleName]
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
		return s.TargetsEnvelope != nil
	}

	_, ok := s.DelegationEnvelopes[roleName]
	return ok
}

func (s *State) HasRuleName(name string) bool {
	return s.ruleNames.Has(name)
}

// preprocess handles several "one time" tasks when the state is first loaded.
// This includes things like loading the set of rule names present in the state,
// checking if it has file rules, etc.
func (s *State) preprocess() error {
	if s.TargetsEnvelope == nil {
		return nil
	}

	s.ruleNames = set.NewSet[string]()

	targetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName, false)
	if err != nil {
		return err
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

	if len(s.DelegationEnvelopes) == 0 {
		return nil
	}

	for delegatedRoleName := range s.DelegationEnvelopes {
		delegatedMetadata, err := s.GetTargetsMetadata(delegatedRoleName, false)
		if err != nil {
			return err
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

func (s *State) getRootVerifier() (*Verifier, error) {
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

	return &Verifier{
		repository: s.repository,
		principals: principals,
		threshold:  threshold,
	}, nil
}

func (s *State) getTargetsVerifier() (*Verifier, error) {
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

	return &Verifier{
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
func loadStateForEntry(repo *gitinterface.Repository, entry *rsl.ReferenceEntry) (*State, error) {
	if entry.RefName != PolicyRef && entry.RefName != PolicyStagingRef {
		return nil, rsl.ErrRSLEntryDoesNotMatchRef
	}

	commitTreeID, err := repo.GetCommitTreeID(entry.TargetID)
	if err != nil {
		return nil, err
	}

	allTreeEntries, err := repo.GetAllFilesInTree(commitTreeID)
	if err != nil {
		return nil, err
	}

	state := &State{repository: repo}

	for name, blobID := range allTreeEntries {
		contents, err := repo.ReadBlob(blobID)
		if err != nil {
			return nil, err
		}

		// We have this conditional because once upon a time we used to store
		// the root keys on disk as well; now we just get them from the root
		// metadata file. We ignore the keys on disk in the old policy states.
		if strings.HasPrefix(name, metadataTreeEntryName+"/") {
			env := &sslibdsse.Envelope{}
			if err := json.Unmarshal(contents, env); err != nil {
				return nil, err
			}

			metadataName := strings.TrimPrefix(name, metadataTreeEntryName+"/")
			switch metadataName {
			case fmt.Sprintf("%s.json", RootRoleName):
				state.RootEnvelope = env

			case fmt.Sprintf("%s.json", TargetsRoleName):
				state.TargetsEnvelope = env

			default:
				if state.DelegationEnvelopes == nil {
					state.DelegationEnvelopes = map[string]*sslibdsse.Envelope{}
				}

				state.DelegationEnvelopes[strings.TrimSuffix(metadataName, ".json")] = env
			}
		}
	}

	if err := state.preprocess(); err != nil {
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
	state.RootPublicKeys = rootPrincipals

	state.githubAppApprovalsTrusted = rootMetadata.IsGitHubAppApprovalTrusted()

	githubAppPrincipals, err := rootMetadata.GetGitHubAppPrincipals()
	if err == nil {
		state.githubAppKeys = githubAppPrincipals
	} else if state.githubAppApprovalsTrusted {
		return nil, tuf.ErrGitHubAppInformationNotFoundInRoot
	}

	return state, nil
}

func verifyRootKeysMatch(keys1, keys2 []tuf.Principal) bool {
	if len(keys1) != len(keys2) {
		return false
	}

	sort.Slice(keys1, func(i, j int) bool {
		return keys1[i].ID() < keys1[j].ID()
	})

	sort.Slice(keys2, func(i, j int) bool {
		return keys2[i].ID() < keys2[j].ID()
	})

	return reflect.DeepEqual(keys1, keys2)
}

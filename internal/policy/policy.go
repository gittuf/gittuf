// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
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

	rootPublicKeysTreeEntryName = "keys"
	metadataTreeEntryName       = "metadata"

	gitReferenceRuleScheme = "git"
	fileRuleScheme         = "file"
)

var (
	ErrMetadataNotFound           = errors.New("unable to find requested metadata file; has it been initialized?")
	ErrInvalidPolicyTree          = errors.New("invalid policy tree structure")
	ErrDanglingDelegationMetadata = errors.New("unreachable targets metadata found")
	ErrNotRSLEntry                = errors.New("RSL entry expected, annotation found instead")
	ErrDelegationNotFound         = errors.New("required delegation entry not found")
	ErrPolicyExists               = errors.New("cannot initialize Policy namespace as it exists already")
	ErrPolicyNotFound             = errors.New("cannot find policy")
	ErrDuplicatedRuleName         = errors.New("two rules with same name found in policy")
	ErrUnableToMatchRootKeys      = errors.New("unable to match root public keys, gittuf policy is in a broken state")
	ErrNotAncestor                = errors.New("cannot apply changes since policy is not an ancestor of the policy staging")
)

// InitializeNamespace creates a git ref for the policy. Initially, the entry
// has a zero hash.
func InitializeNamespace(repo *git.Repository) error {
	for _, name := range []string{PolicyRef, PolicyStagingRef} {
		if ref, err := repo.Reference(plumbing.ReferenceName(name), true); err != nil {
			if !errors.Is(err, plumbing.ErrReferenceNotFound) {
				return err
			}
		} else if !ref.Hash().IsZero() {
			return ErrPolicyExists
		}
	}

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(PolicyStagingRef), plumbing.ZeroHash)); err != nil {
		return err
	}

	return repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(PolicyRef), plumbing.ZeroHash))
}

// State contains the full set of metadata and root keys present in a policy
// state.
type State struct {
	RootEnvelope        *sslibdsse.Envelope
	TargetsEnvelope     *sslibdsse.Envelope
	DelegationEnvelopes map[string]*sslibdsse.Envelope
	RootPublicKeys      []*tuf.Key

	verifiersCache map[string][]*Verifier
	ruleNames      *set.Set[string]
}

type DelegationWithDepth struct {
	Delegation tuf.Delegation
	Depth      int
}

// LoadState returns the State of the repository's policy corresponding to the
// entry. It verifies the root of trust for the state from the initial policy
// entry in the RSL.
func LoadState(ctx context.Context, repo *git.Repository, entry *rsl.ReferenceEntry) (*State, error) {
	firstEntry, _, err := rsl.GetFirstEntry(repo)
	if err != nil {
		return nil, err
	}

	// This assumes the first entry is for the policy ref
	initialState, err := loadStateForEntry(ctx, repo, firstEntry)
	if err != nil {
		return nil, err
	}

	allPolicyEntries, _, err := rsl.GetReferenceEntriesInRangeForRef(repo, firstEntry.ID, entry.ID, PolicyRef)
	if err != nil {
		return nil, err
	}

	if len(allPolicyEntries) == 0 {
		return nil, ErrPolicyNotFound
	}

	slog.Debug(fmt.Sprintf("Trusting root of trust for initial policy '%s'...", firstEntry.ID))
	verifiedState := initialState
	for _, entry := range allPolicyEntries[1:] {
		slog.Debug(fmt.Sprintf("Verifying root of trust for policy '%s'...", entry.ID))
		currentState, err := loadStateForEntry(ctx, repo, entry)
		if err != nil {
			return nil, err
		}

		if err := verifiedState.VerifyNewState(ctx, currentState); err != nil {
			return nil, err
		}

		verifiedState = currentState
	}

	return verifiedState, nil
}

// LoadCurrentState returns the State corresponding to the repository's current
// active policy. It verifies the root of trust for the state starting from the
// initial policy entry in the RSL.
func LoadCurrentState(ctx context.Context, repo *git.Repository, ref string) (*State, error) {
	entry, _, err := rsl.GetLatestReferenceEntryForRef(repo, ref)
	if err != nil {
		return nil, err
	}

	return loadStateForEntry(ctx, repo, entry)
}

// GetStateForCommit scans the RSL to identify the first time a commit was seen
// in the repository. The policy preceding that RSL entry is returned as the
// State to be used for verifying the commit's signature. If the commit hasn't
// been seen in the repository previously, no policy state is returned. Also, no
// error is returned. Identifying the policy in this case is left to the calling
// workflow.
func GetStateForCommit(ctx context.Context, repo *git.Repository, commit *object.Commit) (*State, error) {
	firstSeenEntry, _, err := rsl.GetFirstReferenceEntryForCommit(repo, commit)
	if err != nil {
		if errors.Is(err, rsl.ErrNoRecordOfCommit) {
			return nil, nil
		}
		return nil, err
	}

	commitPolicyEntry, _, err := rsl.GetLatestReferenceEntryForRefBefore(repo, PolicyRef, firstSeenEntry.ID)
	if err != nil {
		return nil, err
	}

	return LoadState(ctx, repo, commitPolicyEntry)
}

// PublicKeys returns all the public keys associated with a state.
func (s *State) PublicKeys() (map[string]*tuf.Key, error) {
	allKeys := map[string]*tuf.Key{}

	// Add root keys
	for _, key := range s.RootPublicKeys {
		key := key
		allKeys[key.KeyID] = key
	}

	// Add keys from the root metadata
	rootMetadata, err := s.GetRootMetadata()
	if err != nil {
		return nil, err
	}
	for keyID, key := range rootMetadata.Keys {
		key := key
		allKeys[keyID] = key
	}

	// Add keys from top level targets metadata
	if s.TargetsEnvelope == nil {
		// Early states where this hasn't been initialized yet
		return nil, err
	}
	targetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		return nil, err
	}
	for keyID, key := range targetsMetadata.Delegations.Keys {
		key := key
		allKeys[keyID] = key
	}

	// Add keys from delegated targets metadata
	for roleName := range s.DelegationEnvelopes {
		delegatedMetadata, err := s.GetTargetsMetadata(roleName)
		if err != nil {
			return nil, err
		}
		for keyID, key := range delegatedMetadata.Delegations.Keys {
			key := key
			allKeys[keyID] = key
		}
	}

	return allKeys, nil
}

// FindPublicKeysForPath identifies the trusted keys for the path. If the path
// protected in gittuf policy, the trusted keys are returned.
//
// Deprecated: use FindVerifiersForPath.
func (s *State) FindPublicKeysForPath(ctx context.Context, path string) ([]*tuf.Key, error) {
	if err := s.Verify(ctx); err != nil {
		return nil, err
	}

	targetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		return nil, err
	}

	allPublicKeys := targetsMetadata.Delegations.Keys
	delegationsQueue := targetsMetadata.Delegations.Roles
	seenRoles := map[string]bool{TargetsRoleName: true}

	trustedKeys := []*tuf.Key{}
	for {
		if len(delegationsQueue) <= 1 {
			return trustedKeys, nil
		}

		delegation := delegationsQueue[0]
		delegationsQueue = delegationsQueue[1:]

		if delegation.Matches(path) {
			for _, keyID := range delegation.KeyIDs {
				key := allPublicKeys[keyID]
				trustedKeys = append(trustedKeys, key)
			}

			if _, seen := seenRoles[delegation.Name]; seen {
				continue
			}

			if s.HasTargetsRole(delegation.Name) {
				delegatedMetadata, err := s.GetTargetsMetadata(delegation.Name)
				if err != nil {
					return nil, err
				}

				seenRoles[delegation.Name] = true

				for keyID, key := range delegatedMetadata.Delegations.Keys {
					allPublicKeys[keyID] = key
				}

				if delegation.Terminating {
					// Remove other delegations from the queue
					delegationsQueue = delegatedMetadata.Delegations.Roles
				} else {
					// Depth first, so newly discovered delegations go first
					// Also, we skip the allow-rule, so we don't include the
					// last element in the delegatedMetadata list.
					delegationsQueue = append(delegatedMetadata.Delegations.Roles[:len(delegatedMetadata.Delegations.Roles)-1], delegationsQueue...)
				}
			}
		}
	}
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
	targetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		return nil, err
	}

	allPublicKeys := targetsMetadata.Delegations.Keys
	// each entry is a list of delegations from a particular metadata file
	groupedDelegations := [][]tuf.Delegation{
		targetsMetadata.Delegations.Roles,
	}

	seenRoles := map[string]bool{TargetsRoleName: true}

	var currentDelegationGroup []tuf.Delegation
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
					name:      delegation.Name,
					keys:      make([]*tuf.Key, 0, len(delegation.KeyIDs)),
					threshold: delegation.Threshold,
				}
				for _, keyID := range delegation.KeyIDs {
					key := allPublicKeys[keyID]
					verifier.keys = append(verifier.keys, key)
				}
				verifiers = append(verifiers, verifier)

				if _, seen := seenRoles[delegation.Name]; seen {
					continue
				}

				if s.HasTargetsRole(delegation.Name) {
					delegatedMetadata, err := s.GetTargetsMetadata(delegation.Name)
					if err != nil {
						return nil, err
					}

					seenRoles[delegation.Name] = true

					for keyID, key := range delegatedMetadata.Delegations.Keys {
						allPublicKeys[keyID] = key
					}

					// Add the current metadata's further delegations upfront to
					// be depth-first
					groupedDelegations = append([][]tuf.Delegation{delegatedMetadata.Delegations.Roles}, groupedDelegations...)

					if delegation.Terminating {
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
	rootKeys, err := s.GetRootKeys()
	if err != nil {
		return err
	}
	if !verifyRootKeysMatch(rootKeys, s.RootPublicKeys) {
		return ErrUnableToMatchRootKeys
	}

	if s.TargetsEnvelope == nil {
		return nil
	}

	targetsVerifier, err := s.getTargetsVerifier()
	if err != nil {
		return err
	}

	if err := targetsVerifier.Verify(ctx, nil, s.TargetsEnvelope); err != nil {
		return err
	}

	targetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		return err
	}

	reachedDelegations := map[string]bool{}
	for delegatedRoleName := range s.DelegationEnvelopes {
		reachedDelegations[delegatedRoleName] = false
	}

	delegationsQueue := targetsMetadata.Delegations.Roles
	delegationKeys := targetsMetadata.Delegations.Keys
	for {
		// The last entry in the queue is always the allow rule, which we don't
		// process during DFS
		if len(delegationsQueue) <= 1 {
			break
		}

		delegation := delegationsQueue[0]
		delegationsQueue = delegationsQueue[1:]

		if s.HasTargetsRole(delegation.Name) {
			reachedDelegations[delegation.Name] = true

			env := s.DelegationEnvelopes[delegation.Name]

			keys := []*tuf.Key{}
			for _, keyID := range delegation.KeyIDs {
				keys = append(keys, delegationKeys[keyID])
			}

			verifier := &Verifier{
				name:      delegation.Name,
				keys:      keys,
				threshold: delegation.Threshold,
			}

			if err := verifier.Verify(ctx, nil, env); err != nil {
				return err
			}

			delegatedMetadata, err := s.GetTargetsMetadata(delegation.Name)
			if err != nil {
				return err
			}

			delegationsQueue = append(delegatedMetadata.Delegations.Roles, delegationsQueue...)
			for keyID, key := range delegatedMetadata.Delegations.Keys {
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
// an RSL entry recording the new tip of the targetRef namespace.
func (s *State) Commit(ctx context.Context, repo *git.Repository, commitMessage string, signCommit bool, targetRef string) error {
	if err := s.Verify(ctx); err != nil {
		return err
	}

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

	metadataEntries := []object.TreeEntry{}
	for name, env := range metadata {
		metadataContents, err := json.Marshal(env)
		if err != nil {
			return err
		}

		blobID, err := gitinterface.WriteBlob(repo, metadataContents)
		if err != nil {
			return err
		}

		metadataEntries = append(metadataEntries, object.TreeEntry{
			Name: fmt.Sprintf("%s.json", name),
			Mode: filemode.Regular,
			Hash: blobID,
		})
	}
	metadataTreeID, err := gitinterface.WriteTree(repo, metadataEntries)
	if err != nil {
		return err
	}

	keysEntries := []object.TreeEntry{}
	for _, key := range s.RootPublicKeys {
		keyContents, err := json.Marshal(key)
		if err != nil {
			return err
		}

		blobID, err := gitinterface.WriteBlob(repo, keyContents)
		if err != nil {
			return err
		}

		keysEntries = append(keysEntries, object.TreeEntry{
			Name: key.KeyID,
			Mode: filemode.Regular,
			Hash: blobID,
		})
	}
	keysTreeID, err := gitinterface.WriteTree(repo, keysEntries)
	if err != nil {
		return err
	}

	policyRootTreeID, err := gitinterface.WriteTree(repo, []object.TreeEntry{
		{
			Name: metadataTreeEntryName,
			Mode: filemode.Dir,
			Hash: metadataTreeID,
		},
		{
			Name: rootPublicKeysTreeEntryName,
			Mode: filemode.Dir,
			Hash: keysTreeID,
		},
	})
	if err != nil {
		return err
	}

	ref, err := repo.Reference(plumbing.ReferenceName(targetRef), true)
	if err != nil {
		return err
	}
	originalCommitID := ref.Hash()

	commitID, err := gitinterface.Commit(repo, policyRootTreeID, targetRef, commitMessage, signCommit)
	if err != nil {
		return err
	}

	// We must reset to original policy commit if err != nil from here onwards.

	if err := rsl.NewReferenceEntry(targetRef, commitID).Commit(repo, signCommit); err != nil {
		return gitinterface.ResetDueToError(err, repo, targetRef, originalCommitID)
	}

	return nil
}

func (s *State) GetRootKeys() ([]*tuf.Key, error) {
	rootMetadata, err := s.GetRootMetadata()
	if err != nil {
		return nil, err
	}

	rootKeys := make([]*tuf.Key, 0, len(rootMetadata.Roles[RootRoleName].KeyIDs))
	for _, keyID := range rootMetadata.Roles[RootRoleName].KeyIDs {
		key, has := rootMetadata.Keys[keyID]
		if !has {
			return nil, ErrRootKeyNil
		}

		rootKeys = append(rootKeys, key)
	}

	return rootKeys, nil
}

// GetRootMetadata returns the deserialized payload of the State's RootEnvelope.
func (s *State) GetRootMetadata() (*tuf.RootMetadata, error) {
	payloadBytes, err := s.RootEnvelope.DecodeB64Payload()
	if err != nil {
		return nil, err
	}

	rootMetadata := &tuf.RootMetadata{}
	if err := json.Unmarshal(payloadBytes, rootMetadata); err != nil {
		return nil, err
	}

	return rootMetadata, nil
}

func (s *State) GetTargetsMetadata(roleName string) (*tuf.TargetsMetadata, error) {
	e := s.TargetsEnvelope
	if roleName != TargetsRoleName {
		env, ok := s.DelegationEnvelopes[roleName]
		if !ok {
			return nil, ErrMetadataNotFound
		}
		e = env
	}
	payloadBytes, err := e.DecodeB64Payload()
	if err != nil {
		return nil, err
	}

	targetsMetadata := &tuf.TargetsMetadata{}
	if err := json.Unmarshal(payloadBytes, targetsMetadata); err != nil {
		return nil, err
	}

	return targetsMetadata, nil
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

func (s *State) loadRuleNames() error {
	if s.TargetsEnvelope == nil {
		return nil
	}

	s.ruleNames = set.NewSet[string]()

	targetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		return err
	}

	for _, rule := range targetsMetadata.Delegations.Roles {
		if rule.Name == AllowRuleName {
			continue
		}

		if s.ruleNames.Has(rule.Name) {
			return ErrDuplicatedRuleName
		}

		s.ruleNames.Add(rule.Name)
	}

	if len(s.DelegationEnvelopes) == 0 {
		return nil
	}

	for delegatedRoleName := range s.DelegationEnvelopes {
		delegatedMetadata, err := s.GetTargetsMetadata(delegatedRoleName)
		if err != nil {
			return err
		}

		for _, rule := range delegatedMetadata.Delegations.Roles {
			if rule.Name == AllowRuleName {
				continue
			}

			if s.ruleNames.Has(rule.Name) {
				return ErrDuplicatedRuleName
			}

			s.ruleNames.Add(rule.Name)
		}
	}

	return nil
}

// ListRules returns a list of all the rules as an array of the delegations in a
// pre order traversal of the delegation tree, with the depth of each
// delegation.
func ListRules(ctx context.Context, repo *git.Repository, targetRef string) ([]*DelegationWithDepth, error) {
	state, err := LoadCurrentState(ctx, repo, targetRef)
	if err != nil {
		return nil, err
	}

	topLevelTargetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		return nil, err
	}

	delegationsToSearch := []*DelegationWithDepth{}
	allDelegations := []*DelegationWithDepth{}

	for _, topLevelDelegation := range topLevelTargetsMetadata.Delegations.Roles {
		if topLevelDelegation.Name == AllowRuleName {
			continue
		}
		delegationsToSearch = append(delegationsToSearch, &DelegationWithDepth{Delegation: topLevelDelegation, Depth: 0})
	}

	seenRoles := map[string]bool{TargetsRoleName: true}

	for len(delegationsToSearch) > 0 {
		currentDelegation := delegationsToSearch[0]
		delegationsToSearch = delegationsToSearch[1:]

		// allDelegations will be the returned list of all the delegations in pre-order traversal, no delegations will be popped off
		allDelegations = append(allDelegations, currentDelegation)

		if _, seen := seenRoles[currentDelegation.Delegation.Name]; seen {
			continue
		}

		if state.HasTargetsRole(currentDelegation.Delegation.Name) {
			currentMetadata, err := state.GetTargetsMetadata(currentDelegation.Delegation.Name)
			if err != nil {
				return nil, err
			}

			seenRoles[currentDelegation.Delegation.Name] = true

			// We construct localDelegations first so that we preserve the order
			// of delegations in currentMetadata in delegationsToSearch
			localDelegations := []*DelegationWithDepth{}
			for _, delegation := range currentMetadata.Delegations.Roles {
				if delegation.Name == AllowRuleName {
					continue
				}
				localDelegations = append(localDelegations, &DelegationWithDepth{Delegation: delegation, Depth: currentDelegation.Depth + 1})
			}

			if len(localDelegations) > 0 {
				delegationsToSearch = append(localDelegations, delegationsToSearch...)
			}
		}
	}

	return allDelegations, nil
}

// hasFileRule returns true if the policy state has a single rule in any targets
// role with the file namespace scheme. Note that this function has no concept
// of role reachability, as it is not invoked for a specific path. So, it might
// return true even if the role in question is not reachable for some path (or
// at all).
func (s *State) hasFileRule() (bool, error) {
	if s.TargetsEnvelope == nil {
		// No top level targets, we don't need to check for delegated roles
		return false, nil
	}

	targetsRole, err := s.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		return false, err
	}

	rolesToCheck := []*tuf.TargetsMetadata{targetsRole}

	// This doesn't consider whether a delegated role is reachable because we
	// don't know what artifact path this is for
	for roleName := range s.DelegationEnvelopes {
		delegatedRole, err := s.GetTargetsMetadata(roleName)
		if err != nil {
			return false, err
		}
		rolesToCheck = append(rolesToCheck, delegatedRole)
	}

	for _, role := range rolesToCheck {
		for _, delegation := range role.Delegations.Roles {
			if delegation.Name == AllowRuleName {
				continue
			}

			for _, path := range delegation.Paths {
				if strings.HasPrefix(path, "file:") {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (s *State) getRootVerifier() (*Verifier, error) {
	rootMetadata, err := s.GetRootMetadata()
	if err != nil {
		return nil, err
	}

	return &Verifier{
		keys:      s.RootPublicKeys,
		threshold: rootMetadata.Roles[RootRoleName].Threshold,
	}, nil
}

// Apply takes valid changes from the policy staging ref, and fast-forward
// merges it into the policy ref. Apply only takes place if the latest state on
// the policy staging ref is valid. This prevents invalid changes to the policy
// taking affect, and allowing new changes, that until signed by multiple users
// would be invalid to be made, by utilizing the policy staging ref.
func Apply(ctx context.Context, repo *git.Repository, policyFiles []string, signRSLEntry bool) error {
	// Get the reference for the PolicyRef
	policyRef, err := repo.Reference(plumbing.ReferenceName(PolicyRef), true)
	if err != nil {
		return fmt.Errorf("failed to get policy reference %s: %w", PolicyRef, err)
	}

	// Get the reference for the PolicyStagingRef
	policyStagingRef, err := repo.Reference(plumbing.ReferenceName(PolicyStagingRef), true)
	if err != nil {
		return fmt.Errorf("failed to get policy staging reference %s: %w", PolicyStagingRef, err)
	}

	// Check if the PolicyStagingRef is ahead of PolicyRef (fast-forward)

	policyStagingCommit, err := gitinterface.GetCommit(repo, policyStagingRef.Hash())
	if err != nil {
		// if there is no tip for the policy staging ref, this means that no change will be made to the policy ref
		return fmt.Errorf("failed to get policy staging tip commit: %w", err)
	}

	policyCommit, err := gitinterface.GetCommit(repo, policyRef.Hash())
	if !errors.Is(err, plumbing.ErrObjectNotFound) {
		if err != nil {
			return err
		}
		// This check ensures that the policy staging branch is a direct forward progression of the policy branch,
		// preventing any overwrites of policy history and maintaining a linear policy evolution, since a
		// fast-forward merge does not work with a non-linear history.

		// This is only being checked if there are no problems finding the tip of the policy ref, since if there
		// is no tip, then it cannot be an ancestor of the tip of the policy staging ref
		isAncestor, err := gitinterface.KnowsCommit(repo, policyStagingCommit.Hash, policyCommit)
		if err != nil {
			return fmt.Errorf("failed to check if policy commit is ancestor of policy staging commit: %w", err)
		}
		if !isAncestor {
			return ErrNotAncestor
		}
	}

	if len(policyFiles) == 1 && policyFiles[0] == "." {
		// using LoadCurrentState to verify if the PolicyStagingRef's latest state is valid
		_, err = LoadCurrentState(ctx, repo, PolicyStagingRef)
		if err != nil {
			return fmt.Errorf("failed to load current state: %w", err)
		}
		// Update the reference for the base to point to the new commit
		newPolicyRef := plumbing.NewHashReference(PolicyRef, policyStagingRef.Hash())
		if err := repo.Storer.SetReference(newPolicyRef); err != nil {
			return fmt.Errorf("failed to set new policy reference: %w", err)
		}

		if err := rsl.NewReferenceEntry(PolicyRef, policyStagingRef.Hash()).Commit(repo, signRSLEntry); err != nil {
			return gitinterface.ResetDueToError(err, repo, PolicyRef, policyRef.Hash())
		}
	} else {
		currentPolicyState, err := LoadCurrentState(ctx, repo, PolicyRef)
		if err != nil {
			return err
		}

		nextPolicy, err := LoadCurrentState(ctx, repo, PolicyRef)
		if err != nil {
			return err
		}

		currentStagingState, err := LoadCurrentState(ctx, repo, PolicyStagingRef)
		if err != nil {
			return err
		}
		// search all policy files we want to apply
		for _, policyFile := range policyFiles {
			// mutate the state based on policy file
			switch policyFile {
			case TargetsRoleName:
				nextPolicy.TargetsEnvelope = currentStagingState.TargetsEnvelope
			case "root-keys":
				nextPolicy.RootPublicKeys = currentStagingState.RootPublicKeys
			default:
				found := false
				for policyFileName, metadata := range currentStagingState.DelegationEnvelopes {
					if policyFileName == policyFile {
						found = true
						nextPolicy.DelegationEnvelopes[policyFileName] = metadata
						break
					}
				}

				if !found {
					return fmt.Errorf("policy item %s was not found", policyFile)
				}
			}
		}

		// verify if the state can be made from the previous one, does this break the state or not?
		if err := currentPolicyState.VerifyNewState(ctx, nextPolicy); err != nil {
			return err
		}
		// commit the new policy into the policy ref, updating the policy state
		if err := nextPolicy.Commit(ctx, repo, "merged policy-staging into policy", signRSLEntry, PolicyRef); err != nil {
			return err
		}
	}
	return nil
}

func (s *State) getTargetsVerifier() (*Verifier, error) {
	rootMetadata, err := s.GetRootMetadata()
	if err != nil {
		return nil, err
	}

	verifier := &Verifier{keys: make([]*tuf.Key, 0, len(rootMetadata.Roles[TargetsRoleName].KeyIDs))}
	for _, keyID := range rootMetadata.Roles[TargetsRoleName].KeyIDs {
		verifier.keys = append(verifier.keys, rootMetadata.Keys[keyID])
	}
	verifier.threshold = rootMetadata.Roles[TargetsRoleName].Threshold

	return verifier, nil
}

// loadStateForEntry returns the State for a specified RSL reference entry for
// the policy namespace. This helper is focused on reading the Git object store
// and loading the policy contents. Typically, LoadCurrentState of LoadState
// must be used. The exception is VerifyRelative... which performs root
// verification between consecutive policy states.
func loadStateForEntry(ctx context.Context, repo *git.Repository, entry *rsl.ReferenceEntry) (*State, error) {
	if entry.RefName != PolicyRef && entry.RefName != PolicyStagingRef {
		return nil, rsl.ErrRSLEntryDoesNotMatchRef
	}

	policyCommit, err := gitinterface.GetCommit(repo, entry.TargetID)
	if err != nil {
		return nil, err
	}

	policyRootTree, err := gitinterface.GetTree(repo, policyCommit.TreeHash)
	if err != nil {
		return nil, err
	}

	if len(policyRootTree.Entries) > 2 {
		return nil, ErrInvalidPolicyTree
	}

	var (
		metadataTreeID plumbing.Hash
		keysTreeID     plumbing.Hash
	)

	for _, e := range policyRootTree.Entries {
		switch e.Name {
		case metadataTreeEntryName:
			metadataTreeID = e.Hash
		case rootPublicKeysTreeEntryName:
			keysTreeID = e.Hash
		default:
			return nil, ErrInvalidPolicyTree
		}
	}

	state := &State{}

	metadataTree, err := gitinterface.GetTree(repo, metadataTreeID)
	if err != nil {
		return nil, err
	}

	keysTree, err := gitinterface.GetTree(repo, keysTreeID)
	if err != nil {
		return nil, err
	}

	for _, entry := range metadataTree.Entries {
		contents, err := gitinterface.ReadBlob(repo, entry.Hash)
		if err != nil {
			return nil, err
		}

		env := &sslibdsse.Envelope{}
		if err := json.Unmarshal(contents, env); err != nil {
			return nil, err
		}

		switch entry.Name {
		case fmt.Sprintf("%s.json", RootRoleName):
			state.RootEnvelope = env
		case fmt.Sprintf("%s.json", TargetsRoleName):
			state.TargetsEnvelope = env
		default:
			if state.DelegationEnvelopes == nil {
				state.DelegationEnvelopes = map[string]*sslibdsse.Envelope{}
			}

			state.DelegationEnvelopes[strings.TrimSuffix(entry.Name, ".json")] = env
		}
	}

	for _, entry := range keysTree.Entries {
		contents, err := gitinterface.ReadBlob(repo, entry.Hash)
		if err != nil {
			return nil, err
		}

		key, err := tuf.LoadKeyFromBytes(contents)
		if err != nil {
			return nil, err
		}

		if state.RootPublicKeys == nil {
			state.RootPublicKeys = []*tuf.Key{}
		}

		state.RootPublicKeys = append(state.RootPublicKeys, key)
	}

	if err := state.loadRuleNames(); err != nil {
		return nil, err
	}

	if err := state.Verify(ctx); err != nil {
		return nil, err
	}

	return state, nil
}

func verifyRootKeysMatch(keys1, keys2 []*tuf.Key) bool {
	if len(keys1) != len(keys2) {
		return false
	}

	sort.Slice(keys1, func(i, j int) bool {
		return keys1[i].KeyID < keys1[j].KeyID
	})

	sort.Slice(keys2, func(i, j int) bool {
		return keys2[i].KeyID < keys2[j].KeyID
	})

	return reflect.DeepEqual(keys1, keys2)
}

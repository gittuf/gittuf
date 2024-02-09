// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
)

// InitializeNamespace creates a git ref for the policy. Initially, the entry
// has a zero hash.
func InitializeNamespace(repo *git.Repository) error {
	for _, name := range []string{PolicyRef /*, PolicyStagingRef*/} {
		if ref, err := repo.Reference(plumbing.ReferenceName(name), true); err != nil {
			if !errors.Is(err, plumbing.ErrReferenceNotFound) {
				return err
			}
		} else if !ref.Hash().IsZero() {
			return ErrPolicyExists
		}
	}

	// Disable PolicyStagingRef until it is actually used
	// https://github.com/gittuf/gittuf/issues/45
	// if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(PolicyStagingRef), plumbing.ZeroHash)); err != nil {
	// 	return err
	// }

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
}

type DelegationWithDepth struct {
	Delegation tuf.Delegation
	Depth      int
}

// LoadState returns the State of the repository's policy corresponding to the
// rslEntryID.
func LoadState(ctx context.Context, repo *git.Repository, rslEntryID plumbing.Hash) (*State, error) {
	entry, err := rsl.GetEntry(repo, rslEntryID)
	if err != nil {
		return nil, err
	}

	refEntry, ok := entry.(*rsl.ReferenceEntry)
	if !ok {
		return nil, ErrNotRSLEntry
	}

	return LoadStateForEntry(ctx, repo, refEntry)
}

// LoadCurrentState returns the State corresponding to the repository's current
// active policy.
func LoadCurrentState(ctx context.Context, repo *git.Repository) (*State, error) {
	entry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
	if err != nil {
		return nil, err
	}

	return LoadStateForEntry(ctx, repo, entry)
}

// LoadStateForEntry returns the State for a specified RSL reference entry for
// the policy namespace.
func LoadStateForEntry(ctx context.Context, repo *git.Repository, entry *rsl.ReferenceEntry) (*State, error) {
	if entry.RefName != PolicyRef {
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

	if err := state.Verify(ctx); err != nil {
		return nil, err
	}

	return state, nil
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

	return LoadStateForEntry(ctx, repo, commitPolicyEntry)
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

// FindAuthorizedSigningKeyIDs traverses the policy metadata to identify the
// keys trusted to sign for the specified role.
//
// Deprecated: diamond delegations are legal in policy. So, role A and role B
// can both independently delegate to role C, and they *don't* need to specify
// the same set of keys / threshold. So, when signing role C, we actually can't
// determine if the keys being used to sign it are valid. It depends strictly on
// how role C is reached, whether via role A or role B. In turn, that depends on
// the exact namespace being verified. In TUF, this issue is known as
// "promiscuous delegations". See:
// https://github.com/theupdateframework/specification/issues/19,
// https://github.com/theupdateframework/specification/issues/214, and
// https://github.com/theupdateframework/python-tuf/issues/660.
func (s *State) FindAuthorizedSigningKeyIDs(ctx context.Context, roleName string) ([]string, error) {
	if err := s.Verify(ctx); err != nil {
		return nil, err
	}

	rootMetadata, err := s.GetRootMetadata()
	if err != nil {
		return nil, err
	}

	if roleName == RootRoleName {
		return rootMetadata.Roles[RootRoleName].KeyIDs, nil
	}

	if roleName == TargetsRoleName {
		if _, ok := rootMetadata.Roles[TargetsRoleName]; !ok {
			return nil, ErrDelegationNotFound
		}

		return rootMetadata.Roles[TargetsRoleName].KeyIDs, nil
	}

	entry, err := s.findDelegationEntry(roleName)
	if err != nil {
		return nil, err
	}

	return entry.KeyIDs, nil
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
func (s *State) FindVerifiersForPath(ctx context.Context, path string) ([]*Verifier, error) {
	if s.verifiersCache == nil {
		s.verifiersCache = map[string][]*Verifier{}
	} else if verifiers, cacheHit := s.verifiersCache[path]; cacheHit {
		// Cache hit for this path in this policy
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
					env := s.DelegationEnvelopes[delegation.Name]
					if err := verifier.Verify(ctx, nil, env); err != nil {
						return nil, err
					}

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

// Verify verifies the signatures of the Root role and the top level Targets
// role if it exists.
func (s *State) Verify(ctx context.Context) error {
	rootVerifier, err := s.getRootVerifier()
	if err != nil {
		return err
	}

	if err := rootVerifier.Verify(ctx, nil, s.RootEnvelope); err != nil {
		return err
	}

	if s.TargetsEnvelope == nil {
		return nil
	}

	targetsVerifier, err := s.getTargetsVerifier()
	if err != nil {
		return err
	}

	return targetsVerifier.Verify(ctx, nil, s.TargetsEnvelope)
}

// Commit verifies and writes the State to the policy namespace. It also creates
// an RSL entry recording the new tip of the policy namespace.
func (s *State) Commit(ctx context.Context, repo *git.Repository, commitMessage string, signCommit bool) error {
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

	ref, err := repo.Reference(plumbing.ReferenceName(PolicyRef), true)
	if err != nil {
		return err
	}
	originalCommitID := ref.Hash()

	commitID, err := gitinterface.Commit(repo, policyRootTreeID, PolicyRef, commitMessage, signCommit)
	if err != nil {
		return err
	}

	// We must reset to original policy commit if err != nil from here onwards.

	if err := rsl.NewReferenceEntry(PolicyRef, commitID).Commit(repo, signCommit); err != nil {
		return gitinterface.ResetDueToError(err, repo, PolicyRef, originalCommitID)
	}

	return nil
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

// ListRules returns a list of all the rules as an array of the delegations in a
// pre order traversal of the delegation tree, with the depth of each
// delegation.
func ListRules(ctx context.Context, repo *git.Repository) ([]*DelegationWithDepth, error) {
	state, err := LoadCurrentState(ctx, repo)
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
	// TODO: validate against the root metadata itself?
	// This eventually goes back to how the very first root is bootstrapped
	// See: https://github.com/gittuf/gittuf/issues/117
	rootMetadata, err := s.GetRootMetadata()
	if err != nil {
		return nil, err
	}

	return &Verifier{
		keys:      s.RootPublicKeys,
		threshold: rootMetadata.Roles[RootRoleName].Threshold,
	}, nil
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

// findDelegationEntry finds the delegation entry for some role in the parent
// role.
//
// Deprecated: diamond delegations are legal in policy. So, role A and role B
// can both independently delegate to role C, and they *don't* need to specify
// the same set of keys / threshold. So, when signing role C, we actually can't
// determine if the keys being used to sign it are valid. It depends strictly on
// how role C is reached, whether via role A or role B. In turn, that depends on
// the exact namespace being verified. In TUF, this issue is known as
// "promiscuous delegations". See:
// https://github.com/theupdateframework/specification/issues/19,
// https://github.com/theupdateframework/specification/issues/214, and
// https://github.com/theupdateframework/python-tuf/issues/660.
func (s *State) findDelegationEntry(roleName string) (*tuf.Delegation, error) {
	topLevelTargetsMetadata, err := s.GetTargetsMetadata(TargetsRoleName)
	if err != nil {
		return nil, err
	}

	delegationTargetsMetadata := map[string]*tuf.TargetsMetadata{}
	for name, env := range s.DelegationEnvelopes {
		targetsMetadata := &tuf.TargetsMetadata{}

		envBytes, err := env.DecodeB64Payload()
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(envBytes, targetsMetadata); err != nil {
			return nil, err
		}
		delegationTargetsMetadata[name] = targetsMetadata
	}

	delegationsQueue := topLevelTargetsMetadata.Delegations.Roles

	seenRoles := map[string]bool{TargetsRoleName: true}

	for {
		if len(delegationsQueue) == 0 {
			return nil, ErrDelegationNotFound
		}

		delegation := &delegationsQueue[0]
		delegationsQueue = delegationsQueue[1:]

		if delegation.Name == roleName {
			return delegation, nil
		}

		if _, seen := seenRoles[delegation.Name]; seen {
			continue
		}

		if s.HasTargetsRole(delegation.Name) {
			delegationsQueue = append(delegationsQueue, delegationTargetsMetadata[delegation.Name].Delegations.Roles...)
			seenRoles[delegation.Name] = true
		}
	}
}

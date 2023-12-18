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
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
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
)

var (
	ErrMetadataNotFound           = errors.New("unable to find requested metadata file; has it been initialized?")
	ErrInvalidPolicyTree          = errors.New("invalid policy tree structure")
	ErrDanglingDelegationMetadata = errors.New("unreachable targets metadata found")
	ErrNotRSLEntry                = errors.New("RSL entry expected, annotation found instead")
	ErrDelegationNotFound         = errors.New("required delegation entry not found")
)

var ErrPolicyExists = errors.New("cannot initialize Policy namespace as it exists already")

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

	// TODO: verify root from original state? We have consecutive verification
	// in place elsewhere.
	rootVerifier := state.getRootVerifier()
	if err := rootVerifier.Verify(ctx, nil, state.RootEnvelope); err != nil {
		return nil, err
	}

	if state.TargetsEnvelope != nil {
		targetsVerifier, err := state.getTargetsVerifier()
		if err != nil {
			return nil, err
		}

		if err := targetsVerifier.Verify(ctx, nil, state.TargetsEnvelope); err != nil {
			return nil, err
		}
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
// Deprecated: we want to avoid promiscuous delegations where multiple roles may
// delegate to the same role and we can't clarify up front which role's trusted
// keys we must use. We only know if a delegated role is trusted when we're
// actively walking the graph for a specific path. See:
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

			if s.HasTargetsRole(delegation.Name) {
				delegatedMetadata, err := s.GetTargetsMetadata(delegation.Name)
				if err != nil {
					return nil, err
				}
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

	var currentDelegationGroup []tuf.Delegation
	verifiers := []*Verifier{}
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

				if s.HasTargetsRole(delegation.Name) {
					env := s.DelegationEnvelopes[delegation.Name]
					if err := verifier.Verify(ctx, nil, env); err != nil {
						return nil, err
					}

					delegatedMetadata, err := s.GetTargetsMetadata(delegation.Name)
					if err != nil {
						return nil, err
					}
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

// Verify performs a self-contained verification of all the metadata in the
// State starting from the Root. Any metadata that is unreachable in the
// delegations graph returns an error.
//
// Deprecated: we want to avoid promiscuous delegations where multiple roles may
// delegate to the same role and we can't clarify up front which role's trusted
// keys we must use. We only know if a delegated role is trusted when we're
// actively walking the graph for a specific path. See:
// https://github.com/theupdateframework/specification/issues/19,
// https://github.com/theupdateframework/specification/issues/214, and
// https://github.com/theupdateframework/python-tuf/issues/660.
func (s *State) Verify(ctx context.Context) error {
	rootVerifiers := []sslibdsse.Verifier{}
	for _, k := range s.RootPublicKeys {
		sv, err := signerverifier.NewSignerVerifierFromTUFKey(k)
		if err != nil {
			return err
		}

		rootVerifiers = append(rootVerifiers, sv)
	}
	if err := dsse.VerifyEnvelope(ctx, s.RootEnvelope, rootVerifiers, len(rootVerifiers)); err != nil {
		return err
	}

	if s.TargetsEnvelope == nil {
		return nil
	}

	rootMetadata := &tuf.RootMetadata{}
	rootContents, err := s.RootEnvelope.DecodeB64Payload()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(rootContents, rootMetadata); err != nil {
		return err
	}

	targetsVerifiers := []sslibdsse.Verifier{}
	for _, keyID := range rootMetadata.Roles[TargetsRoleName].KeyIDs {
		key := rootMetadata.Keys[keyID]
		sv, err := signerverifier.NewSignerVerifierFromTUFKey(key)
		if err != nil {
			return err
		}

		targetsVerifiers = append(targetsVerifiers, sv)
	}
	if err := dsse.VerifyEnvelope(ctx, s.TargetsEnvelope, targetsVerifiers, rootMetadata.Roles[TargetsRoleName].Threshold); err != nil {
		return err
	}

	if len(s.DelegationEnvelopes) == 0 {
		return nil
	}

	delegationEnvelopes := map[string]*sslibdsse.Envelope{}
	for k, v := range s.DelegationEnvelopes {
		delegationEnvelopes[k] = v
	}

	targetsMetadata := &tuf.TargetsMetadata{}
	targetsContents, err := s.TargetsEnvelope.DecodeB64Payload()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(targetsContents, targetsMetadata); err != nil {
		return err
	}

	if err := targetsMetadata.Validate(); err != nil {
		return err
	}

	// Note: If targetsMetadata.Delegations == nil while delegationEnvelopes is
	// not empty, we probably want to error out. This should panic.
	delegationKeys := targetsMetadata.Delegations.Keys
	delegationsQueue := targetsMetadata.Delegations.Roles

	// We can likely process top level targets and all delegated envelopes in
	// the loop below by combining the two but this separated model seems easier
	// to reason about. Else, we define a custom starting delegation from root
	// to targets in the queue and start this loop from there.

	for {
		if len(delegationsQueue) == 0 {
			break
		}

		delegation := delegationsQueue[0]
		delegationsQueue = delegationsQueue[1:]

		delegationEnvelope, ok := delegationEnvelopes[delegation.Name]
		if !ok {
			// Delegation does not have an envelope to verify
			continue
		}
		delete(delegationEnvelopes, delegation.Name)

		delegationVerifiers := make([]sslibdsse.Verifier, 0, len(delegation.KeyIDs))
		for _, keyID := range delegation.KeyIDs {
			key := delegationKeys[keyID]
			sv, err := signerverifier.NewSignerVerifierFromTUFKey(key)
			if err != nil {
				return err
			}

			delegationVerifiers = append(delegationVerifiers, sv)
		}

		if err := dsse.VerifyEnvelope(ctx, delegationEnvelope, delegationVerifiers, delegation.Threshold); err != nil {
			return err
		}

		delegationMetadata := &tuf.TargetsMetadata{}
		delegationContents, err := delegationEnvelope.DecodeB64Payload()
		if err != nil {
			return err
		}
		if err := json.Unmarshal(delegationContents, delegationMetadata); err != nil {
			return err
		}

		if err := delegationMetadata.Validate(); err != nil {
			return err
		}

		if delegationMetadata.Delegations == nil {
			continue
		}

		for keyID, key := range delegationMetadata.Delegations.Keys {
			delegationKeys[keyID] = key
		}

		delegationsQueue = append(delegationsQueue, delegationMetadata.Delegations.Roles...)
	}

	if len(delegationEnvelopes) != 0 {
		return ErrDanglingDelegationMetadata
	}

	return nil
}

// Commit verifies and writes the State to the policy namespace. It also creates
// an RSL entry recording the new tip of the policy namespace.
func (s *State) Commit(ctx context.Context, repo *git.Repository, commitMessage string, signCommit bool) error {
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

func (s *State) getRootVerifier() *Verifier {
	return &Verifier{
		keys:      s.RootPublicKeys,
		threshold: len(s.RootPublicKeys),
	}
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
// Deprecated: we want to avoid promiscuous delegations where multiple roles may
// delegate to the same role and we can't clarify up front which role's trusted
// keys we must use. We only know if a delegated role is trusted when we're
// actively walking the graph for a specific path. See:
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

	for {
		if len(delegationsQueue) == 0 {
			return nil, ErrDelegationNotFound
		}

		delegation := &delegationsQueue[0]
		delegationsQueue = delegationsQueue[1:]

		if delegation.Name == roleName {
			return delegation, nil
		}

		if s.HasTargetsRole(delegation.Name) {
			delegationsQueue = append(delegationsQueue, delegationTargetsMetadata[delegation.Name].Delegations.Roles...)
		}
	}
}

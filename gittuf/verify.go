package gittuf

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/adityasaky/gittuf/internal/gitstore"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/secure-systems-lab/go-securesystemslib/cjson"
	tufdata "github.com/theupdateframework/go-tuf/data"
	tufkeys "github.com/theupdateframework/go-tuf/pkg/keys"
)

const AllowRule = "allow-*"

/*
VerifyTrustedStates compares two TUF states of the repository, stateA and
stateB, and validates if the repository can move from stateA to stateB.
Both states are specified as tips of the gittuf namespace. Note that this API
does NOT update the contents of the gittuf namespace.
*/
func VerifyTrustedStates(target string, stateA string, stateB string) error {
	if stateA == stateB {
		return nil
	}

	// Verify the ref is in valid git format
	if !IsValidGitTarget(target) {
		return fmt.Errorf("specified ref '%s' is not in valid git format", target)
	}

	repoRoot, err := GetRepoRootDir()
	if err != nil {
		return err
	}

	stateARepo, err := gitstore.LoadAtState(repoRoot, stateA)
	if err != nil {
		return err
	}

	// FIXME: what if target / rule didn't exist before and no metadata exists?
	stateARefTree, err := getStateTree(stateARepo, target)
	if err != nil {
		return err
	}

	stateBRepo, err := gitstore.LoadAtState(repoRoot, stateB)
	if err != nil {
		return err
	}
	stateBRefTree, err := getStateTree(stateBRepo, target)
	if err != nil {
		return err
	}

	// Get keys used signing role of target in stateB
	_, roleName, err := getTargetsRoleForTarget(stateBRepo, target)
	if err != nil {
		return err
	}
	roleBytes, err := stateBRepo.GetCurrentMetadataBytes(roleName)
	if err != nil {
		return err
	}
	var roleSigned tufdata.Signed
	err = json.Unmarshal(roleBytes, &roleSigned)
	if err != nil {
		return err
	}
	usedKeyIDs := []string{}
	for _, sig := range roleSigned.Signatures {
		usedKeyIDs = append(usedKeyIDs, sig.KeyID)
	}

	changes, err := stateARefTree.Diff(stateBRefTree)
	if err != nil {
		return err
	}

	return validateChanges(stateARepo, changes, usedKeyIDs)
}

/*
VerifyState checks that a target has the hash specified in the TUF delegations tree.
*/
func VerifyState(store *gitstore.GitStore, target string) error {
	state := store.State()
	activeID, err := getCurrentCommitID(target)
	if err != nil {
		return err
	}

	currentTargets, role, err := getTargetsRoleForTarget(state, target)
	if err != nil {
		return err
	}

	currentTargetsID := currentTargets.Targets[target].Hashes["sha1"]
	if !reflect.DeepEqual(currentTargetsID, activeID) {
		return fmt.Errorf("role %s has recorded different hash value %s from current hash %s", role, currentTargetsID.String(), activeID.String())
	}

	lastTrustedStateID, err := store.LastTrusted(target)
	if err != nil {
		return err
	}
	lastTrustedState, err := store.SpecificState(lastTrustedStateID)
	if err != nil {
		return err
	}
	lastTrustedTargets, role, err := getTargetsRoleForTarget(lastTrustedState, target)
	if err != nil {
		return err
	}
	lastTrustedTargetsID := lastTrustedTargets.Targets[target].Hashes["sha1"]
	if !reflect.DeepEqual(lastTrustedTargetsID, activeID) {
		return fmt.Errorf("role %s has recorded different hash value %s from current hash %s", role, lastTrustedTargetsID.String(), activeID.String())
	}

	return nil
}

func getCurrentCommitID(target string) (tufdata.HexBytes, error) {
	// We check if target has the form git:...
	// In future, if multiple schemes are supported, this function can dispatch
	// to different parsers.

	if !IsValidGitTarget(target) {
		return tufdata.HexBytes{}, fmt.Errorf("%s is not a Git object", target)
	}

	refName, refType, err := ParseGitTarget(target)
	if err != nil {
		return tufdata.HexBytes{}, err
	}

	return GetTipCommitIDForRef(refName, refType)
}

func getTargetsRoleForTarget(state *gitstore.State, target string) (*tufdata.Targets, string, error) {
	topLevelTargets, err := loadTopLevelTargets(state)
	if err != nil {
		return &tufdata.Targets{}, "", err
	}

	refName, _, err := ParseGitTarget(target)
	if err != nil {
		return &tufdata.Targets{}, "", err
	}

	allowRuleHit := false
	acceptedKeys := map[string]*tufdata.PublicKey{}
	for _, delegation := range topLevelTargets.Delegations.Roles {
		if delegation.Name == AllowRule {
			// there are no restrictions on who can sign
			allowRuleHit = true
			break
		}
		matches, err := delegation.MatchesPath(target)
		if err != nil {
			return &tufdata.Targets{}, "", err
		}
		if matches {
			acceptedKeys = topLevelTargets.Delegations.Keys
		}
	}

	contents, err := state.GetCurrentMetadataBytes(refName)
	if err != nil {
		return &tufdata.Targets{}, "", err
	}

	var s *tufdata.Signed
	err = json.Unmarshal(contents, &s)
	if err != nil {
		return &tufdata.Targets{}, "", err
	}

	var role *tufdata.Targets
	err = json.Unmarshal(s.Signed, &role)
	if err != nil {
		return &tufdata.Targets{}, "", err
	}

	if !allowRuleHit {
		msg, err := cjson.EncodeCanonical(role)
		if err != nil {
			return &tufdata.Targets{}, "", err
		}

		for _, signature := range s.Signatures {
			verifier, err := tufkeys.GetVerifier(acceptedKeys[signature.KeyID])
			if err != nil {
				return &tufdata.Targets{}, "", err
			}
			err = verifier.Verify(msg, signature.Signature)
			if err != nil {
				return &tufdata.Targets{}, "", err
			}
		}
	}

	return role, refName, nil
}

func getStateTree(metadataRepo *gitstore.State, target string) (*object.Tree, error) {
	mainRepo, err := GetRepoHandler()
	if err != nil {
		return &object.Tree{}, err
	}

	stateTargets, _, err := getTargetsRoleForTarget(metadataRepo, target)
	if err != nil {
		return &object.Tree{}, err
	}

	// This is NOT in the gittuf namespace
	stateEntryHash := plumbing.NewHash(stateTargets.Targets[target].Hashes["sha1"].String())
	stateRefCommit, err := mainRepo.CommitObject(stateEntryHash)
	if err != nil {
		return &object.Tree{}, err
	}
	return mainRepo.TreeObject(stateRefCommit.TreeHash)
}

func validateUsedKeyIDs(authorizedKeyIDs map[string]tufdata.PublicKey, usedKeyIDs []string) bool {
	set := map[string]bool{}
	for k := range authorizedKeyIDs {
		set[k] = true
	}
	for _, k := range usedKeyIDs {
		if _, ok := set[k]; !ok {
			return false
		}
	}
	return true
}

func validateRule(ruleState *gitstore.State, path string, usedKeyIDs []string) error {
	keys, threshold, err := ExpectedSignersForTarget(ruleState, path)
	if err != nil {
		return err
	}
	if threshold == 0 {
		return nil
	}

	// TODO: threshold
	if !validateUsedKeyIDs(keys, usedKeyIDs) {
		return fmt.Errorf("unauthorized change to file %s", path)
	}

	return nil
}

func validateChanges(ruleState *gitstore.State, changes object.Changes, usedKeyIDs []string) error {
	for _, c := range changes {
		// For each change to a file, we want to verify that the policy allows
		// the keys that were used to sign changes for the file.

		// First, we get the delegations entry for the target. If we end up at
		// the catch all rule, we move on to the next change.

		// Once we have a delegations entry, we get a list of keys authorized
		// to sign for the target. We then check if the keys used were part of
		// this authorized set.

		// If the change includes a rename, we follow the above rules for the
		// original name AND the new name. This ensures that a rename does not
		// result in a file being written to a protected namespace.

		if len(c.From.Name) > 0 {
			if err := validateRule(ruleState, c.From.Name, usedKeyIDs); err != nil {
				return err
			}
		}

		if c.From.Name != c.To.Name && len(c.To.Name) > 0 {
			if err := validateRule(ruleState, c.To.Name, usedKeyIDs); err != nil {
				return err
			}
		}

	}
	return nil
}

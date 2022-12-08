package gittuf

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adityasaky/gittuf/internal/gitstore"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	tufdata "github.com/theupdateframework/go-tuf/data"
	tuftargets "github.com/theupdateframework/go-tuf/pkg/targets"
	tufverify "github.com/theupdateframework/go-tuf/verify"
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
	if !strings.HasPrefix(target, "git:") {
		return fmt.Errorf("specified ref '%s' is not in valid git format", target)
	}

	repoRoot, err := GetRepoRootDir()
	if err != nil {
		return err
	}

	stateARepo, err := gitstore.LoadRepositoryAtState(repoRoot, stateA)
	if err != nil {
		return err
	}

	// FIXME: what if target / rule didn't exist before and no metadata exists?
	stateARefTree, err := getStateTree(stateARepo, target)
	if err != nil {
		return err
	}

	stateBRepo, err := gitstore.LoadRepositoryAtState(repoRoot, stateB)
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
	roleBytes := stateBRepo.GetCurrentFileBytes(roleName)
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
func VerifyState(repo *gitstore.Repository, target string) error {
	currentHash, err := getCurrentHash(target)
	if err != nil {
		return err
	}

	targets, role, err := getTargetsRoleForTarget(repo, target)
	if err != nil {
		return err
	}

	entryHash := targets.Targets[target].Hashes["sha1"].String()
	if currentHash != entryHash {
		return fmt.Errorf("role %s has different hash value %s from current hash %s", role, entryHash, currentHash)
	}

	return nil
}

func InitializeTopLevelDB(repo *gitstore.Repository) (*tufverify.DB, error) {
	db := tufverify.NewDB()

	rootRole, err := loadRoot(repo)
	if err != nil {
		return db, err
	}

	for id, key := range rootRole.Keys {
		if err := db.AddKey(id, key); err != nil {
			return db, err
		}
	}

	for name, role := range rootRole.Roles {
		if err := db.AddRole(name, role); err != nil {
			return db, err
		}
	}

	return db, nil
}

func InitializeDBUntilRole(repo *gitstore.Repository, roleName string) (*tufverify.DB, error) {
	db, err := InitializeTopLevelDB(repo)
	if err != nil {
		return db, err
	}

	if roleName == "targets" {
		// The top level DB has that covered
		return db, nil
	}

	toBeChecked := []string{"targets"}

	for {
		if len(toBeChecked) == 0 {
			if len(roleName) != 0 {
				return db, fmt.Errorf("role %s not found", roleName)
			} else {
				// We found every reachable role
				return db, nil
			}
		}

		current := toBeChecked[0]
		toBeChecked = toBeChecked[1:]

		targets, err := loadTargets(repo, current, db)
		if err != nil {
			return db, err
		}

		if targets.Delegations == nil {
			continue
		}

		for id, key := range targets.Delegations.Keys {
			db.AddKey(id, key)
		}

		for _, d := range targets.Delegations.Roles {
			db.AddRole(d.Name, &tufdata.Role{
				KeyIDs:    d.KeyIDs,
				Threshold: d.Threshold,
			})
			if d.Name == roleName {
				return db, nil
			}
			toBeChecked = append(toBeChecked, d.Name)
		}
	}
}

func getCurrentHash(target string) (string, error) {
	// We check if target has the form git:...
	// In future, if multiple schemes are supported, this function can dispatch
	// to different parsers.

	if !strings.HasPrefix(target, "git:") {
		return "", fmt.Errorf("%s is not a Git object", target)
	}

	refName, refType, err := ParseGitTarget(target)
	if err != nil {
		return "", err
	}

	commitID, err := GetTipCommitIDForRef(refName, refType)
	if err != nil {
		return "", err
	}

	return string(commitID), nil
}

func getTargetsRoleForTarget(repo *gitstore.Repository, target string) (*tufdata.Targets, string, error) {
	db, err := InitializeTopLevelDB(repo)
	if err != nil {
		return &tufdata.Targets{}, "", err
	}

	topLevelTargets, err := loadTargets(repo, "targets", db)
	if err != nil {
		return &tufdata.Targets{}, "", err
	}

	if _, ok := topLevelTargets.Targets[target]; ok {
		return topLevelTargets, "targets", nil
	}

	iterator, err := tuftargets.NewDelegationsIterator(target, db)
	if err != nil {
		return &tufdata.Targets{}, "", err
	}

	for {
		d, ok := iterator.Next()
		if !ok {
			return &tufdata.Targets{}, "",
				fmt.Errorf("delegation not found for target %s", target)
		}

		delegatedRole, err := loadTargets(repo, d.Delegatee.Name, d.DB)
		if err != nil {
			return &tufdata.Targets{}, "", err
		}

		if _, ok := delegatedRole.Targets[target]; ok {
			return delegatedRole, d.Delegatee.Name, nil
		}

		if delegatedRole.Delegations != nil {
			newDB, err := tufverify.NewDBFromDelegations(delegatedRole.Delegations)
			if err != nil {
				return &tufdata.Targets{}, "", err
			}
			err = iterator.Add(delegatedRole.Delegations.Roles, d.Delegatee.Name, newDB)
			if err != nil {
				return &tufdata.Targets{}, "", err
			}
		}
	}
}

func getStateTree(metadataRepo *gitstore.Repository, target string) (*object.Tree, error) {
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

func getDelegationForTarget(repo *gitstore.Repository, target string) (tufdata.DelegatedRole, error) {
	db, err := InitializeTopLevelDB(repo)
	if err != nil {
		return tufdata.DelegatedRole{}, err
	}

	iterator, err := tuftargets.NewDelegationsIterator(target, db)
	if err != nil {
		return tufdata.DelegatedRole{}, err
	}

	for {
		d, ok := iterator.Next()
		if !ok {
			return tufdata.DelegatedRole{},
				fmt.Errorf("delegation not found for target %s", target)
		}

		match, err := d.Delegatee.MatchesPath(target)
		if err != nil {
			return tufdata.DelegatedRole{}, err
		}
		if match {
			return d.Delegatee, nil
		}

		delegatedRole, err := loadTargets(repo, d.Delegatee.Name, d.DB)
		if err != nil {
			return tufdata.DelegatedRole{}, err
		}

		if delegatedRole.Delegations != nil {
			newDB, err := tufverify.NewDBFromDelegations(delegatedRole.Delegations)
			if err != nil {
				return tufdata.DelegatedRole{}, err
			}
			err = iterator.Add(delegatedRole.Delegations.Roles, d.Delegatee.Name, newDB)
			if err != nil {
				return tufdata.DelegatedRole{}, err
			}
		}
	}
}

func validateUsedKeyIDs(authorizedKeyIDs []string, usedKeyIDs []string) bool {
	set := map[string]bool{}
	for _, k := range authorizedKeyIDs {
		set[k] = true
	}
	for _, k := range usedKeyIDs {
		if _, ok := set[k]; !ok {
			return false
		}
	}
	return true
}

func validateRule(ruleRepo *gitstore.Repository, path string, usedKeyIDs []string) error {
	ruleInA, err := getDelegationForTarget(ruleRepo, path)
	if err != nil {
		return err
	}
	if ruleInA.Name == AllowRule {
		return nil
	}

	// TODO: threshold
	if !validateUsedKeyIDs(ruleInA.KeyIDs, usedKeyIDs) {
		return fmt.Errorf("unauthorized change to file %s", path)
	}

	return nil
}

func validateChanges(policyRepo *gitstore.Repository, changes object.Changes, usedKeyIDs []string) error {
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
			if err := validateRule(policyRepo, c.From.Name, usedKeyIDs); err != nil {
				return err
			}
		}

		if c.From.Name != c.To.Name && len(c.To.Name) > 0 {
			if err := validateRule(policyRepo, c.To.Name, usedKeyIDs); err != nil {
				return err
			}
		}

	}
	return nil
}

package gittuf

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/adityasaky/gittuf/internal/gitstore"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sirupsen/logrus"
	tufdata "github.com/theupdateframework/go-tuf/data"
)

func Commit(state *gitstore.State, branchName string, keys []tufdata.PrivateKey, expires time.Time, gitArgs ...string) (tufdata.Signed, string, error) {
	// TODO: Should `commit` check for updated metadata on a remote?

	// TODO: do we need URI IDs for targetName?
	targetName, _ := CreateGitTarget(branchName, GitBranchRef) // we're passing in BranchRef explicitly, we can skip the error check

	keyIDsToUse := []string{}
	for _, k := range keys {
		pubKey, err := GetEd25519PublicKeyFromPrivateKey(&k)
		if err != nil {
			return tufdata.Signed{}, "", err
		}
		keyIDsToUse = append(keyIDsToUse, pubKey.IDs()...)
	}
	err := verifyStagedFilesCanBeModified(state, keyIDsToUse)
	if err != nil {
		return tufdata.Signed{}, "", err
	}

	expectedKeys, expectedThreshold, err := ExpectedSignersForTarget(state, targetName)
	if err != nil {
		return tufdata.Signed{}, "", err
	}

	if len(keys) < expectedThreshold {
		return tufdata.Signed{}, "", fmt.Errorf("not enough keys to meet threshold for ref %s", branchName)
	}

	for _, k := range keyIDsToUse {
		if _, ok := expectedKeys[k]; !ok {
			return tufdata.Signed{}, "", fmt.Errorf("key %s not authorized to sign for ref %s", k, branchName)
		}
	}

	// Create a commit and get its identifier
	commitID, err := createCommit(gitArgs)
	if err != nil {
		// commit will have been undone in createCommit already
		return tufdata.Signed{}, "", err
	}

	var targetsRole *tufdata.Targets
	if state.HasFile(branchName) {
		if threshold == 0 {
			targetsRole, err = loadSpecificTargetsWithoutVerification(state, branchName)
		} else {
			targetsRole, err = loadSpecificTargets(state, branchName, keys, threshold)
		}
		if err != nil {
			return tufdata.Signed{}, "", UndoLastCommit(err)
		}
	} else {
		targetsRole = tufdata.NewTargets()
	}

	// Add entry to role
	targetsRole.Targets[targetName] = tufdata.TargetFileMeta{
		FileMeta: tufdata.FileMeta{
			Length: 1,
			Hashes: map[string]tufdata.HexBytes{
				"sha1": commitID,
			},
		},
	}

	// Update version number
	targetsRole.Version++

	// Update expiry
	targetsRole.Expires = expires

	signedRoleMb, err := generateAndSignMbFromStruct(targetsRole, keys)
	if err != nil {
		return tufdata.Signed{}, "", UndoLastCommit(err)
	}

	return signedRoleMb, targetName, nil
}

func verifyStagedFilesCanBeModified(state *gitstore.State, keyIDs []string) error {
	mainRepo, err := GetRepoHandler()
	if err != nil {
		return err
	}

	worktree, err := mainRepo.Worktree()
	if err != nil {
		return err
	}
	worktreeStatus, err := worktree.Status()
	if err != nil {
		return err
	}

	changes := []*object.Change{}

	for path, status := range worktreeStatus {
		logrus.Debugf("Checking if %s can be modified", path)
		if status.Staging == git.Modified {
			to := path
			if len(status.Extra) > 0 {
				to = status.Extra
			}
			changes = append(changes, &object.Change{
				// The other fields in ChangeEntry are never used.
				// We should reconsider the signature of validateChanges.
				From: object.ChangeEntry{
					Name: path,
				},
				To: object.ChangeEntry{
					Name: to,
				},
			})
		}
	}

	return validateChanges(state, changes, keyIDs)
}

func createCommit(gitArgs []string) (tufdata.HexBytes, error) {
	logrus.Debug("Creating commit")

	cmd := exec.Command("git", append([]string{"commit"}, gitArgs...)...)
	err := cmd.Run()
	if err != nil {
		return tufdata.HexBytes{}, err
	}

	// Get ID of commit we just created
	commitID, err := GetHEADCommitID()
	if err != nil {
		return tufdata.HexBytes{}, UndoLastCommit(err)
	}

	logrus.Debug("Created commit", commitID.String())
	return commitID, nil
}

package gittuf

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/adityasaky/gittuf/internal/gitstore"
	"github.com/sirupsen/logrus"
	tufdata "github.com/theupdateframework/go-tuf/data"
)

func Commit(repo *gitstore.Repository, role string, keys []tufdata.PrivateKey, expires time.Time, gitArgs ...string) (tufdata.Signed, error) {
	// TODO: Should `commit` check for updated metadata on a remote?

	cmd := exec.Command("git", "symbolic-ref", "HEAD")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return tufdata.Signed{}, err
	}

	branchSplit := strings.Split(strings.Trim(stdout.String(), "\n"), "/")
	branchName := branchSplit[len(branchSplit)-1]
	targetName := fmt.Sprintf("git:branch=%s", branchName)

	// TODO: Verify staged files can be modified by specified role
	verifyStagedFilesCanBeModified(repo)

	// Create a commit and get its identifier
	commitID, err := createCommit(gitArgs)
	if err != nil {
		// commit will have been undone in createCommit already
		return tufdata.Signed{}, err
	}
	// go-tuf expects hashes to be represented as HexBytes
	commitHB, err := getHashHexBytes(commitID)
	if err != nil {
		return tufdata.Signed{}, UndoCommit(err)
	}

	var targetsRole *tufdata.Targets
	if repo.HasFile(role) {
		db, err := InitializeDBUntilRole(repo, role)
		if err != nil {
			return tufdata.Signed{}, err
		}
		targetsRole, err = loadTargets(repo, role, db)
		if err != nil {
			return tufdata.Signed{}, UndoCommit(err)
		}
	} else {
		targetsRole = tufdata.NewTargets()
	}

	// Add entry to role
	targetsRole.Targets[targetName] = tufdata.TargetFileMeta{
		FileMeta: tufdata.FileMeta{
			Length: 1,
			Hashes: map[string]tufdata.HexBytes{
				"sha1": commitHB,
			},
		},
	}

	// Update version number
	targetsRole.Version++

	// Update expiry
	targetsRole.Expires = expires

	signedRoleMb, err := generateAndSignMbFromStruct(targetsRole, keys)
	if err != nil {
		return tufdata.Signed{}, UndoCommit(err)
	}

	return signedRoleMb, nil
}

func verifyStagedFilesCanBeModified(repo *gitstore.Repository) error {
	cmd := exec.Command("git", "diff", "--staged", "--name-only")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return err
	}
	stagedFilePaths := strings.Split(strings.Trim(stdout.String(), "\n"), "\n")

	db, err := InitializeTopLevelDB(repo)
	if err != nil {
		return err
	}

	logrus.Debug("Created TUF verification database", db)

	for _, target := range stagedFilePaths {
		logrus.Debugf("Checking if %s can be modified", target)
		// 	_, err = tuftargets.NewDelegationsIterator(target, db)
		// 	if err != nil {
		// 		return err
		// 	}
	}

	return nil
}

func createCommit(gitArgs []string) ([]byte, error) {
	logrus.Debug("Creating commit")

	commitID := []byte{}
	command := []string{"commit"}
	command = append(command, gitArgs...)
	cmd := exec.Command("git", command...)
	err := cmd.Run()
	if err != nil {
		return commitID, err
	}
	cmd = exec.Command("git", "rev-parse", "HEAD")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return commitID, UndoCommit(err)
	}
	commitID = stdout.Bytes()
	commitID = commitID[0 : len(commitID)-1]

	logrus.Debug("Created commit", string(commitID))

	return commitID, nil
}

func UndoCommit(cause error) error {
	logrus.Debug("Undoing last commit due to error")
	cmd := exec.Command("git", "reset", "--soft", "HEAD~1")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error undoing commit: %w, in turn triggered due to %w", err, cause)
	}
	return cause
}

func getHashHexBytes(hash []byte) (tufdata.HexBytes, error) {
	hb := make(tufdata.HexBytes, hex.DecodedLen(len(hash)))
	_, err := hex.Decode(hb, hash)
	return hb, err
}

package gittuf

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/adityasaky/gittuf/internal/gitstore"
	tuftargets "github.com/theupdateframework/go-tuf/pkg/targets"
	tufverify "github.com/theupdateframework/go-tuf/verify"
)

/*
Verify checks that a target has the hash specified in the TUF delegations tree.
*/
func Verify(repo *gitstore.Repository, target string) error {
	sha1Hash, err := getCurrentHash(target)
	if err != nil {
		return err
	}

	db, err := InitializeTopLevelDB(repo)
	if err != nil {
		return err
	}

	topLevelTargets, err := loadTargets(repo, "targets", db)
	if err != nil {
		return err
	}

	if t, ok := topLevelTargets.Targets[target]; ok {
		expectedCommitID := t.Hashes["sha1"].String()
		if expectedCommitID == sha1Hash {
			return nil
		}
		// We return without checking delegations of top level in this instance
		return fmt.Errorf("top level targets role has different hash value %s", expectedCommitID)
	}

	iterator, err := tuftargets.NewDelegationsIterator(target, db)
	if err != nil {
		return err
	}

	for {
		d, ok := iterator.Next()
		if !ok {
			return fmt.Errorf("delegation not found for target %s", target)
		}

		// TODO: Pass DB here
		delegatedRole, err := loadTargets(repo, d.Delegatee.Name, d.DB)
		if err != nil {
			return err
		}

		if t, ok := delegatedRole.Targets[target]; ok {
			expectedCommitID := t.Hashes["sha1"].String()
			if expectedCommitID != sha1Hash {
				return fmt.Errorf("role %s has different hash value %s from current hash %s", d.Delegatee.Name, expectedCommitID, sha1Hash)
			}

			// Should we continue searching for other delegations?
			// Recent Writer means that a false match should fail
			break
		}

		if delegatedRole.Delegations != nil {
			newDB, err := tufverify.NewDBFromDelegations(delegatedRole.Delegations)
			if err != nil {
				return err
			}
			err = iterator.Add(delegatedRole.Delegations.Roles, d.Delegatee.Name, newDB)
			if err != nil {
				return err
			}
		}
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

func getCurrentHash(target string) (string, error) {
	currentHash := ""
	// First, we check if target has the form git:
	if strings.HasPrefix(target, "git:") {
		// Which git namespace?
		// Actually, does it matter?
		u, err := url.Parse(target)
		if err != nil {
			return "", err
		}
		// TODO: Fix this up, we need a better parser
		objectName := strings.Split(u.Opaque, "=")[1]

		cmd := exec.Command("git", "rev-parse", objectName)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		err = cmd.Run()
		if err != nil {
			return "", err
		}
		currentHash = strings.Trim(stdout.String(), "\n")
	} else {
		return "", fmt.Errorf("%s is not a Git object", target)
	}
	return currentHash, nil
}

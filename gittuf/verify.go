package gittuf

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/adityasaky/gittuf/internal/gitstore"
	tufdata "github.com/theupdateframework/go-tuf/data"
	tuftargets "github.com/theupdateframework/go-tuf/pkg/targets"
	tufverify "github.com/theupdateframework/go-tuf/verify"
)

/*
VerifyState checks that a target has the hash specified in the TUF delegations tree.
*/
func VerifyState(repo *gitstore.Repository, target string) error {
	currentHash, err := getCurrentHash(target)
	if err != nil {
		return err
	}

	targetEntry, role, err := getTargetEntryForTarget(repo, target)
	if err != nil {
		return err
	}

	entryHash := targetEntry.Hashes["sha1"].String()
	if currentHash != targetEntry.Hashes["sha1"].String() {
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
			return db, fmt.Errorf("role %s not found", roleName)
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

func getTargetEntryForTarget(repo *gitstore.Repository, target string) (tufdata.TargetFileMeta, string, error) {
	db, err := InitializeTopLevelDB(repo)
	if err != nil {
		return tufdata.TargetFileMeta{}, "", err
	}

	topLevelTargets, err := loadTargets(repo, "targets", db)
	if err != nil {
		return tufdata.TargetFileMeta{}, "", err
	}

	if t, ok := topLevelTargets.Targets[target]; ok {
		return t, "targets", nil
	}

	iterator, err := tuftargets.NewDelegationsIterator(target, db)
	if err != nil {
		return tufdata.TargetFileMeta{}, "", err
	}

	for {
		d, ok := iterator.Next()
		if !ok {
			return tufdata.TargetFileMeta{}, "",
				fmt.Errorf("delegation not found for target %s", target)
		}

		delegatedRole, err := loadTargets(repo, d.Delegatee.Name, d.DB)
		if err != nil {
			return tufdata.TargetFileMeta{}, "", err
		}

		if t, ok := delegatedRole.Targets[target]; ok {
			return t, d.Delegatee.Name, nil
		}

		if delegatedRole.Delegations != nil {
			newDB, err := tufverify.NewDBFromDelegations(delegatedRole.Delegations)
			if err != nil {
				return tufdata.TargetFileMeta{}, "", err
			}
			err = iterator.Add(delegatedRole.Delegations.Roles, d.Delegatee.Name, newDB)
			if err != nil {
				return tufdata.TargetFileMeta{}, "", err
			}
		}
	}
}

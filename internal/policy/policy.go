package policy

import (
	"errors"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

const (
	PolicyRef        = "refs/gittuf/policy"
	PolicyStagingRef = "refs/gittuf/policy-staging"
)

var ErrPolicyExists = errors.New("cannot initialize Policy namespace as it exists already")

// InitializeNamespace creates a git ref for the policy. Initially, the entry
// has a zero hash.
func InitializeNamespace(repo *git.Repository) error {
	for _, name := range []string{PolicyRef, PolicyStagingRef} {
		if _, err := repo.Reference(plumbing.ReferenceName(name), true); err != nil {
			if !errors.Is(err, plumbing.ErrReferenceNotFound) {
				return err
			}
		} else {
			return ErrPolicyExists
		}
	}

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(PolicyRef), plumbing.ZeroHash)); err != nil {
		return err
	}

	return repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(PolicyStagingRef), plumbing.ZeroHash))
}

package policy

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

const (
	PolicyRef        = "refs/gittuf/policy"
	PolicyStagingRef = "refs/gittuf/policy-staging"
)

// InitializeNamespace creates a git ref for the policy. Initially, the entry
// has a zero hash.
func InitializeNamespace(repo *git.Repository) error {
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(PolicyRef), plumbing.ZeroHash)); err != nil {
		return err
	}

	return repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(PolicyStagingRef), plumbing.ZeroHash))
}

package repository

import (
	"errors"

	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5"
)

var (
	ErrUnauthorizedKey    = errors.New("unauthorized key presented when updating gittuf metadata")
	ErrCannotReinitialize = errors.New("cannot reinitialize metadata, it exists already")
)

type Repository struct {
	r *git.Repository
}

func LoadRepository() (*Repository, error) {
	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, err
	}

	return &Repository{
		r: repo,
	}, nil
}

func (r *Repository) InitializeNamespaces() error {
	if err := rsl.InitializeNamespace(r.r); err != nil {
		return err
	}

	return policy.InitializeNamespace(r.r)
}

func isKeyAuthorized(authorizedKeyIDs []string, keyID string) bool {
	for _, k := range authorizedKeyIDs {
		if k == keyID {
			return true
		}
	}
	return false
}

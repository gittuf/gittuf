// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
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
	slog.Debug("Loading Git repository...")

	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, err
	}

	return &Repository{
		r: repo,
	}, nil
}

func (r *Repository) InitializeNamespaces() error {
	slog.Debug(fmt.Sprintf("Initializing RSL reference '%s'...", rsl.Ref))
	if err := rsl.InitializeNamespace(r.r); err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Initializing attestations reference '%s'...", attestations.Ref))
	if err := attestations.InitializeNamespace(r.r); err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Initializing policy reference '%s'...", policy.PolicyRef))
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

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"errors"
	"log/slog"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/tuf"
)

var (
	ErrUnauthorizedKey    = errors.New("unauthorized key presented when updating gittuf metadata")
	ErrCannotReinitialize = errors.New("cannot reinitialize metadata, it exists already")
)

type Repository struct {
	r *gitinterface.Repository
}

func (r *Repository) GetGitRepository() *gitinterface.Repository {
	return r.r
}

func LoadRepository() (*Repository, error) {
	slog.Debug("Loading Git repository...")

	repo, err := gitinterface.LoadRepository()
	if err != nil {
		return nil, err
	}

	return &Repository{
		r: repo,
	}, nil
}

func isKeyAuthorized(authorizedKeyIDs []tuf.Principal, keyID string) bool {
	for _, k := range authorizedKeyIDs {
		if k.ID() == keyID {
			return true
		}
	}
	return false
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"errors"
	"log/slog"

	repositoryopts "github.com/gittuf/gittuf/experimental/gittuf/options/repository"
	"github.com/gittuf/gittuf/internal/gitinterface"
	gitinterfaceopts "github.com/gittuf/gittuf/internal/gitinterface/options/gitinterface"
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

func LoadRepository(opts ...repositoryopts.Option) (*Repository, error) {
	slog.Debug("Loading Git repository...")

	options := &repositoryopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	repo, err := gitinterface.LoadRepository(gitinterfaceopts.WithRepositoryPath(options.RepositoryPath))
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

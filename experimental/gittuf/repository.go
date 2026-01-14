// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/tuf"
)

const (
	DebugModeKey = "GITTUF_DEBUG"
)

var (
	ErrUnauthorizedKey    = errors.New("unauthorized key presented when updating gittuf metadata")
	ErrCannotReinitialize = errors.New("cannot reinitialize metadata, it exists already")
)

// InDebugMode returns true if gittuf is currently in debug mode.
func InDebugMode() bool {
	return os.Getenv(DebugModeKey) == "1"
}

type Repository struct {
	r *gitinterface.Repository
}

func (r *Repository) GetGitRepository() *gitinterface.Repository {
	return r.r
}

func LoadRepository(repositoryPath string) (*Repository, error) {
	if InDebugMode() {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
	}

	slog.Debug(fmt.Sprintf("Loading Git repository from '%s'...", repositoryPath))

	if repositoryPath == "" {
		return nil, gitinterface.ErrRepositoryPathNotSpecified
	}

	repo, err := gitinterface.LoadRepository(repositoryPath)
	if err != nil {
		return nil, err
	}

	config, err := repo.GetGitConfig()
	if err != nil {
		return nil, err
	}

	// This supports gpg clients other than "gpg"
	// See internal/signerverifier/gpg/gpg.go
	if config["gpg.program"] != "" {
		os.Setenv("GITTUF_GPG_PROGRAM", config["gpg.program"])
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

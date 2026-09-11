// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/gittuf/gittuf/internal/gogitstore"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
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
	r *gogitstore.Storer
}

// GetGitRepository returns the git binary backend. Use GetStorer instead for
// anything taking a gitstore.Storer, so it uses the selected backend.
func (r *Repository) GetGitRepository() *gitinterface.Repository {
	return r.r.Repository
}

// GetStorer returns the repository's storage backend.
func (r *Repository) GetStorer() gitstore.Storer {
	return r.r
}

// newStorer wraps repo with the selected backend, attaching a trace if one was
// requested.
func newStorer(repo *gitinterface.Repository) *gogitstore.Storer {
	backend := storer.resolve()
	enableGoGit := backend == StorerBackendGoGit

	if !storer.tracing() {
		return gogitstore.New(repo, enableGoGit)
	}

	trace := gogitstore.NewTrace()

	activeTrace.mu.Lock()
	activeTrace.trace = trace
	activeTrace.repo = repo
	activeTrace.backend = backend
	activeTrace.mu.Unlock()

	return gogitstore.NewWithTrace(repo, enableGoGit, trace)
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

	return &Repository{
		r: newStorer(repo),
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

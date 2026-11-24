// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/go-git/go-git/v6/plumbing/transport"
)

var (
	ErrCloningRepository          = errors.New("unable to clone repository")
	ErrDirExists                  = errors.New("directory exists")
	ErrCannotDetermineCloneDir    = errors.New("unable to guess a directory name from the remote URL, please specify one")
	ErrExpectedRootKeysDoNotMatch = errors.Join(ErrCloningRepository, errors.New("cloned root keys do not match the expected keys"))
)

// Clone wraps a typical git clone invocation, fetching gittuf refs in addition
// to the standard refs. It performs a verification of the RSL against the
// specified HEAD after cloning the repository. expectedRootKeyRefs are key
// references as accepted by LoadPublicKey; they are loaded after cloning so
// that the cloned repository's Git config (e.g., gpg.program) is used.
// TODO: resolve how root keys are trusted / bootstrapped.
func Clone(ctx context.Context, remoteURL, dir, initialBranch string, expectedRootKeyRefs []string, bare bool) (*Repository, error) {
	slog.Debug("Cloning remote repository", "remoteURL", remoteURL)

	if dir == "" {
		cloneDir, err := determineCloneDir(remoteURL)
		if err != nil {
			return nil, errors.Join(ErrCloningRepository, err)
		}
		dir = cloneDir
	}

	slog.Debug("Checking if local directory exists for repository...")
	_, err := os.Stat(dir)
	if err == nil {
		return nil, errors.Join(ErrCloningRepository, ErrDirExists)
	} else if !os.IsNotExist(err) {
		return nil, errors.Join(ErrCloningRepository, err)
	}

	if err := os.Mkdir(dir, 0755); err != nil {
		return nil, errors.Join(ErrCloningRepository, err)
	}

	refs := []string{"refs/gittuf/*"}

	slog.Debug("Cloning repository...")
	r, err := gitinterface.CloneAndFetchRepository(remoteURL, dir, initialBranch, refs, bare)
	if err != nil {
		if e := os.RemoveAll(dir); e != nil {
			return nil, errors.Join(ErrCloningRepository, err, e)
		}
		return nil, errors.Join(ErrCloningRepository, err)
	}
	head, err := r.GetSymbolicReferenceTarget("HEAD")
	if err != nil {
		return nil, errors.Join(ErrCloningRepository, err)
	}

	repository := &Repository{r: newStorer(r)}

	if len(expectedRootKeyRefs) > 0 {
		expectedRootKeys := make([]tuf.Principal, 0, len(expectedRootKeyRefs))
		for _, keyRef := range expectedRootKeyRefs {
			key, err := LoadPublicKey(repository, keyRef)
			if err != nil {
				if e := os.RemoveAll(dir); e != nil {
					return nil, errors.Join(ErrCloningRepository, err, e)
				}
				return nil, errors.Join(ErrCloningRepository, err)
			}
			expectedRootKeys = append(expectedRootKeys, key)
		}

		slog.Debug("Verifying if root keys are expected root keys...")

		sort.Slice(expectedRootKeys, func(i, j int) bool {
			return expectedRootKeys[i].ID() < expectedRootKeys[j].ID()
		})

		state, err := policy.LoadFirstState(ctx, repository.GetStorer())
		if err != nil {
			return repository, errors.Join(ErrCloningRepository, err)
		}
		rootKeys, err := state.GetRootKeys()
		if err != nil {
			return repository, errors.Join(ErrCloningRepository, err)
		}

		// We sort the root keys so that we can check if the root keys array match's the expected root key array
		sort.Slice(rootKeys, func(i, j int) bool {
			return rootKeys[i].ID() < rootKeys[j].ID()
		})

		if len(rootKeys) != len(expectedRootKeys) {
			return repository, ErrExpectedRootKeysDoNotMatch
		}
		if !reflect.DeepEqual(rootKeys, expectedRootKeys) {
			return repository, ErrExpectedRootKeysDoNotMatch
		}
	}
	slog.Debug("Verifying HEAD...")
	return repository, repository.VerifyRef(ctx, head)
}

// determineCloneDir returns the directory name to clone remoteURL into when
// none is specified. Like Git, it uses the last path component without ".git"
// (so "/path/to/repo/.git" and "host:repo.git" are both cloned into "repo"),
// and falls back to the hostname when the URL has no path. If neither gives a
// usable name, it returns ErrCannotDetermineCloneDir.
//
// See the <directory> option in https://git-scm.com/docs/git-clone and
// git_url_basename() in Git's dir.c:
// https://github.com/git/git/blob/77904e546de3bd940621092ecec47a88d0cede5d/dir.c#L3290
func determineCloneDir(remoteURL string) (string, error) {
	endpoint, err := transport.ParseURL(strings.TrimSpace(remoteURL))
	if err != nil {
		return "", err
	}

	// Local Windows paths keep their backslashes, so normalize them first
	urlPath := strings.ReplaceAll(endpoint.Path, "\\", "/")
	urlPath = strings.TrimSuffix(strings.TrimRight(urlPath, "/"), "/.git")

	dir := strings.TrimSuffix(path.Base(urlPath), ".git")
	if dir == "." || dir == "/" {
		dir = endpoint.Hostname()
	}

	if dir == "" || dir == ".." {
		return "", ErrCannotDetermineCloneDir
	}

	return dir, nil
}

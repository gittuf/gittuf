// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
)

var (
	ErrCloningRepository          = errors.New("unable to clone repository")
	ErrDirExists                  = errors.New("directory exists")
	ErrExpectedRootKeysDoNotMatch = errors.Join(ErrCloningRepository, errors.New("cloned root keys do not match the expected keys"))
)

// Clone wraps a typical git clone invocation, fetching gittuf refs in addition
// to the standard refs. It performs a verification of the RSL against the
// specified HEAD after cloning the repository.
// TODO: resolve how root keys are trusted / bootstrapped.
func Clone(ctx context.Context, remoteURL, dir, initialBranch string, expectedRootKeys []*tuf.Key) (*Repository, error) {
	slog.Debug(fmt.Sprintf("Cloning from '%s'...", remoteURL))

	if dir == "" {
		// FIXME: my understanding is backslashes are not used in URLs but I haven't dived into the RFCs to check yet
		modifiedURL := strings.ReplaceAll(remoteURL, "\\", "/")
		modifiedURL = strings.TrimRight(strings.TrimSpace(modifiedURL), "/") // Trim spaces and trailing slashes if any

		split := strings.Split(modifiedURL, "/")
		dir = strings.TrimSuffix(split[len(split)-1], ".git")
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
	r, err := gitinterface.CloneAndFetchRepository(remoteURL, dir, initialBranch, refs)
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

	repository := &Repository{r: r}

	if len(expectedRootKeys) > 0 {
		slog.Debug("Verifying if root keys are expected root keys...")

		sort.Slice(expectedRootKeys, func(i, j int) bool {
			return expectedRootKeys[i].KeyID < expectedRootKeys[j].KeyID
		})

		state, err := policy.LoadFirstState(ctx, r)
		if err != nil {
			return repository, errors.Join(ErrCloningRepository, err)
		}
		rootKeys, err := state.GetRootKeys()
		if err != nil {
			return repository, errors.Join(ErrCloningRepository, err)
		}

		// We sort the root keys so that we can check if the root keys array match's the expected root key array
		sort.Slice(rootKeys, func(i, j int) bool {
			return rootKeys[i].KeyID < rootKeys[j].KeyID
		})

		if len(rootKeys) != len(expectedRootKeys) {
			return repository, ErrExpectedRootKeysDoNotMatch
		}
		if !reflect.DeepEqual(rootKeys, expectedRootKeys) {
			return repository, ErrExpectedRootKeysDoNotMatch
		}
	}

	slog.Debug("Verifying HEAD...")
	return repository, repository.VerifyRef(ctx, head, false)
}

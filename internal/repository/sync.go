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
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5/plumbing"
)

var (
	ErrCloningRepository            = errors.New("unable to clone repository")
	ErrDirExists                    = errors.New("directory exists")
	ClonedAndExpectedKeysDoNotMatch = errors.New(ErrCloningRepository.Error() + ", " + "cloned root keys do not match the expected keys.")
)

// Clone wraps a typical git clone invocation, fetching gittuf refs in addition
// to the standard refs. It performs a verification of the RSL against the
// specified HEAD after cloning the repository. If called with the expected
// root keys, it will verify that the cloned root keys match the expected ones.
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

	refs := []string{rsl.Ref, policy.PolicyRef}

	slog.Debug("Cloning repository...")
	r, err := gitinterface.CloneAndFetch(ctx, remoteURL, dir, initialBranch, refs)
	if err != nil {
		if e := os.RemoveAll(dir); e != nil {
			return nil, errors.Join(ErrCloningRepository, err, e)
		}
		return nil, errors.Join(ErrCloningRepository, err)
	}
	head, err := r.Reference(plumbing.HEAD, false)
	if err != nil {
		return nil, errors.Join(ErrCloningRepository, err)
	}

	sort.Slice(expectedRootKeys, func(i, j int) bool {
		return expectedRootKeys[i].KeyID < expectedRootKeys[j].KeyID
	})

	repository := &Repository{r: r}

	slog.Debug("Verifying HEAD...")
	if len(expectedRootKeys) > 0 {
		slog.Debug("Loading embedded root keys...")

		firstRSLEntry, _, _ := rsl.GetFirstEntry(r)

		state, err := policy.LoadState(ctx, r, firstRSLEntry.GetID())
		if err != nil {
			return repository, errors.Join(ErrCloningRepository, err)
		}

		if len(state.GetRootKeys()) != len(expectedRootKeys) {
			expectedRootKeysIDs := []string{}

			for _, key := range expectedRootKeys {
				expectedRootKeysIDs = append(expectedRootKeysIDs, key.KeyID)
			}

			return repository, ClonedAndExpectedKeysDoNotMatch
		}

		slog.Debug("Verifying if root keys are expected root keys...")
		for keyid := range state.GetRootKeys() {
			if !reflect.DeepEqual(state.GetRootKeys()[keyid], expectedRootKeys[keyid]) {
				expectedRootKeysIDs := []string{}
				for _, key := range expectedRootKeys {
					expectedRootKeysIDs = append(expectedRootKeysIDs, key.KeyID)
				}

				return repository, ClonedAndExpectedKeysDoNotMatch
			}
		}
	}
	return repository, repository.VerifyRef(ctx, head.Target().String(), true)
}

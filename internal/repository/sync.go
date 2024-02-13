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
	ErrCloningRepository = errors.New("unable to clone repository")
	ErrDirExists         = errors.New("directory exists")
)

// Clone wraps a typical git clone invocation, fetching gittuf refs in addition
// to the standard refs. It performs a verification of the RSL against the
// specified HEAD after cloning the repository. If called with the expected
// root keys, it will verify that the cloned root keys match the expected ones.
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
		state, err := policy.LoadCurrentState(ctx, r)
		if err != nil {
			return repository, errors.Join(ErrCloningRepository, err)
		}

		rootMetadata, err := state.GetRootMetadata()
		if err != nil {
			return repository, errors.Join(ErrCloningRepository, err)
		}

		if len(rootMetadata.Roles[policy.RootRoleName].KeyIDs) != len(expectedRootKeys) {
			expectedRootKeysIds := []string{}

			for _, key := range expectedRootKeys {
				expectedRootKeysIds = append(expectedRootKeysIds, key.KeyID)
			}

			err := fmt.Errorf("cloned root keys do not match the expected keys.\n Expected keys = %s, \n Actual Keys = %s", expectedRootKeysIds, rootMetadata.Roles[policy.RootRoleName].KeyIDs)
			return repository, errors.Join(ErrCloningRepository, err)
		}

		for keyIndex := range expectedRootKeys {
			expectedRootKeysIds := []string{}

			for _, key := range expectedRootKeys {
				expectedRootKeysIds = append(expectedRootKeysIds, key.KeyID)
			}

			if !reflect.DeepEqual(rootMetadata.Keys[rootMetadata.Roles[policy.RootRoleName].KeyIDs[keyIndex]], expectedRootKeys[keyIndex]) {
				err := fmt.Errorf("cloned root keys do not match the expected keys.\n Expected keys = %s, \n Actual Keys = %s", expectedRootKeysIds, rootMetadata.Roles[policy.RootRoleName].KeyIDs)
				return repository, errors.Join(ErrCloningRepository, err)
			}
		}
	}
	return repository, repository.VerifyRef(ctx, head.Target().String(), true)
}

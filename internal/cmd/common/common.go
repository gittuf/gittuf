// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common //nolint:revive

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

var ErrSigningKeyNotSet = errors.New("required flag \"signing-key\" not set")

// ResolveRevision resolves a revision string (full hash, short hash, Git ref, or
// gittuf policy ref) to its target commit ID.
func ResolveRevision(storer gitstore.Storer, revision string) (githash.Hash, error) {
	commitID, err := gitinterface.NewHash(revision)
	if err == nil {
		return commitID, nil
	}

	commitID, err = storer.GetReference(revision)
	if err == nil {
		return commitID, nil
	}

	if !strings.HasPrefix(revision, "refs/gittuf/") {
		gittufCommitID, gittufErr := storer.GetReference("refs/gittuf/" + revision)
		if gittufErr == nil {
			return gittufCommitID, nil
		}
	}

	return nil, fmt.Errorf("unable to resolve revision '%s': %w", revision, err)
}

// PublicKeys is a custom type to represent a list of paths
type PublicKeys []string

// String implements part of the pflag.Value interface.
func (p *PublicKeys) String() string {
	return strings.Join(*p, ", ")
}

// Set implements part of the pflag.Value interface.
func (p *PublicKeys) Set(value string) error {
	*p = append(*p, value)
	return nil
}

// Type implements part of the pflag.Value interface.
func (p *PublicKeys) Type() string {
	return "public-keys"
}

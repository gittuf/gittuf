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

var (
	ErrSigningKeyNotSet = errors.New("required flag \"signing-key\" not set")
	ErrInvalidRevision  = errors.New("invalid revision: must be a full or short-form commit hash")
)

// ResolveRevision resolves a commit hash (full or short-form) to its full
// Git object ID. It rejects Git references and non-hexadecimal revision strings.
func ResolveRevision(storer gitstore.Storer, revision string) (githash.Hash, error) {
	commitID, err := gitinterface.NewHash(revision)
	if err == nil {
		return commitID, nil
	}

	if !isHex(revision) || len(revision) < 4 || len(revision) > 64 {
		return nil, fmt.Errorf("unable to resolve revision '%s': %w", revision, ErrInvalidRevision)
	}

	commitID, err = storer.GetReference(revision)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve revision '%s': %w", revision, err)
	}

	return commitID, nil
}

func isHex(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
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

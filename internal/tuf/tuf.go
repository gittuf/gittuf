// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tuf

// This package defines gittuf's take on TUF metadata. There are some minor
// changes, such as the addition of `custom` to delegation entries. Some of it,
// however, is inspired by or cloned from the go-tuf implementation.

import (
	"errors"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
)

const (
	// RootRoleName defines the expected name for the gittuf root of trust.
	RootRoleName = "root"

	// TargetsRoleName defines the expected name for the top level gittuf policy file.
	TargetsRoleName = "targets"
)

var (
	ErrCannotMeetThreshold       = errors.New("insufficient keys to meet threshold")
	ErrTargetsNotEmpty           = errors.New("`targets` field in gittuf Targets metadata must be empty")
	ErrDuplicatedRuleName        = errors.New("two rules with same name found in policy")
	ErrRootKeyNil                = errors.New("root key not found")
	ErrRootMetadataNil           = errors.New("rootMetadata is nil")
	ErrTargetsMetadataNil        = errors.New("targetsMetadata not found")
	ErrTargetsKeyNil             = errors.New("targetsKey is nil")
	ErrGitHubAppKeyNil           = errors.New("app key is nil")
	ErrKeyIDEmpty                = errors.New("keyID is empty")
	ErrCannotManipulateAllowRule = errors.New("cannot change in-built gittuf-allow-rule")
	ErrRuleNotFound              = errors.New("cannot find rule entry")
	ErrMissingRules              = errors.New("some rules are missing")
)

// Key defines the structure for how public keys are stored in TUF metadata.
type Key = signerverifier.SSLibKey

// Role records common characteristics recorded in a role entry in Root metadata
// and in a delegation entry.
type Role struct {
	KeyIDs    *set.Set[string] `json:"keyids"`
	Threshold int              `json:"threshold"`
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package loadpolicy

import (
	"crypto"

	"github.com/gittuf/gittuf/pkg/githash"
)

type Options struct {
	EntryID         githash.Hash
	AtEntry         bool
	InitialRootKeys []crypto.PublicKey
}

type Option func(o *Options)

// WithEntry loads the policy recorded by one RSL policy entry instead of the
// latest one. entryID must name a reference entry for the policy ref that is
// reachable from the RSL.
func WithEntry(entryID githash.Hash) Option {
	return func(o *Options) {
		o.EntryID = entryID
		o.AtEntry = true
	}
}

// WithInitialRootKeys anchors the loaded history to a known first root of trust.
// Without it, the first root in the RSL is trusted on sight.
//
// Every key must have signed the first root. To anchor a root with a threshold
// below its key count, pass only the keys that signed it.
func WithInitialRootKeys(keys ...crypto.PublicKey) Option {
	return func(o *Options) {
		o.InitialRootKeys = keys
	}
}

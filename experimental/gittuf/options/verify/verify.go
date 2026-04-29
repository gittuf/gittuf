// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verify

import "github.com/gittuf/gittuf/internal/tuf"

type Options struct {
	RefNameOverride  string
	LatestOnly       bool
	ExpectedRootKeys []tuf.Principal
}

type Option func(o *Options)

func WithOverrideRefName(refNameOverride string) Option {
	return func(o *Options) {
		o.RefNameOverride = refNameOverride
	}
}

func WithLatestOnly() Option {
	return func(o *Options) {
		o.LatestOnly = true
	}
}

func WithExpectedRootKeys(principals []tuf.Principal) Option {
	return func(o *Options) {
		o.ExpectedRootKeys = principals
	}
}

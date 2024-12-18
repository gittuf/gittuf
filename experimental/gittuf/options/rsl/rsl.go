// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

type Options struct {
	RefNameOverride       string
	SkipCheckForDuplicate bool
}

type Option func(o *Options)

func WithOverrideRefName(refNameOverride string) Option {
	return func(o *Options) {
		o.RefNameOverride = refNameOverride
	}
}

// WithSkipCheckForDuplicateEntry indicates that the RSL entry creation must not
// check if the latest entry for the reference has the same target ID.
func WithSkipCheckForDuplicateEntry() Option {
	return func(o *Options) {
		o.SkipCheckForDuplicate = true
	}
}

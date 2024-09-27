// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

type Options struct {
	RefNameOverride string
}

type Option func(o *Options)

func WithOverrideRefName(refNameOverride string) Option {
	return func(o *Options) {
		o.RefNameOverride = refNameOverride
	}
}

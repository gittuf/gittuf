// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verify

type Options struct {
	RefNameOverride   string
	LatestOnly        bool
	AutomatedRecovery bool
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

func WithAutomatedRecovery() Option {
	return func(o *Options) {
		o.AutomatedRecovery = true
	}
}

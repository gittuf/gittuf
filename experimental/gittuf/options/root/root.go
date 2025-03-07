// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

type Options struct {
	RepositoryLocation string
	CreateRSLEntry     bool
}

type Option func(o *Options)

func WithRepositoryLocation(location string) Option {
	return func(o *Options) {
		o.RepositoryLocation = location
	}
}

func WithRSLEntry() Option {
	return func(o *Options) {
		o.CreateRSLEntry = true
	}
}

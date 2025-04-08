// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package repository

type Options struct {
	RepositoryPath string
}

type Option func(o *Options)

func WithRepositoryPath(path string) Option {
	return func(o *Options) {
		o.RepositoryPath = path
	}
}

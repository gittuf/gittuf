// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

type RepositoryOptions struct {
	RepositoryPath string
}

type RepositoryOption func(*RepositoryOptions)

func WithRepositoryPath(repositoryPath string) RepositoryOption {
	return func(o *RepositoryOptions) {
		o.RepositoryPath = repositoryPath
	}
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"path"
	"strings"

	"github.com/jonboulle/clockwork"
)

const DefaultRemoteName = "origin"

type FetchOptions struct {
	Depth int
}

type FetchOption func(*FetchOptions)

func WithFetchDepth(depth int) FetchOption {
	return func(o *FetchOptions) {
		o.Depth = depth
	}
}

func (r *Repository) PushRefSpec(remoteName string, refSpecs []string) error {
	args := []string{"push", remoteName}
	args = append(args, refSpecs...)

	_, err := r.executor(args...).executeString()
	if err != nil {
		return fmt.Errorf("unable to push: %w", err)
	}

	return nil
}

func (r *Repository) Push(remoteName string, refs []string) error {
	refSpecs := make([]string, 0, len(refs))
	for _, ref := range refs {
		refSpec, err := r.RefSpec(ref, "", true)
		if err != nil {
			return err
		}
		refSpecs = append(refSpecs, refSpec)
	}

	return r.PushRefSpec(remoteName, refSpecs)
}

func (r *Repository) FetchRefSpec(remoteName string, refSpecs []string, opts ...FetchOption) error {
	options := &FetchOptions{}
	for _, fn := range opts {
		fn(options)
	}

	args := []string{"fetch"}

	if options.Depth != 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", options.Depth))
	}

	args = append(args, remoteName)
	args = append(args, refSpecs...)

	_, err := r.executor(args...).executeString()
	if err != nil {
		return fmt.Errorf("unable to fetch: %w", err)
	}

	return nil
}

func (r *Repository) Fetch(remoteName string, refs []string, fastForwardOnly bool, opts ...FetchOption) error {
	refSpecs := make([]string, 0, len(refs))
	for _, ref := range refs {
		refSpec, err := r.RefSpec(ref, "", fastForwardOnly)
		if err != nil {
			return err
		}
		refSpecs = append(refSpecs, refSpec)
	}

	return r.FetchRefSpec(remoteName, refSpecs, opts...)
}

func (r *Repository) FetchObject(remoteName string, objectID Hash) error {
	args := []string{"fetch", remoteName, objectID.String()}
	_, err := r.executor(args...).executeString()
	if err != nil {
		return fmt.Errorf("unable to fetch object: %w", err)
	}

	return nil
}

func CloneAndFetchRepository(remoteURL, dir, initialBranch string, refs []string, bare bool) (*Repository, error) {
	if dir == "" {
		return nil, fmt.Errorf("target directory must be specified")
	}

	repo := &Repository{clock: clockwork.NewRealClock()}

	args := []string{"clone", remoteURL}
	if initialBranch != "" {
		initialBranch = strings.TrimPrefix(initialBranch, BranchRefPrefix)
		args = append(args, "--branch", initialBranch)
	}
	args = append(args, dir)

	if bare {
		args = append(args, "--bare")
		repo.gitDirPath = dir
	} else {
		repo.gitDirPath = path.Join(dir, ".git")
	}

	_, stdErr, err := repo.executor(args...).execute()
	if err != nil {
		return nil, fmt.Errorf("unable to clone repository: %s", stdErr)
	}

	return repo, repo.Fetch(DefaultRemoteName, refs, true)
}

func (r *Repository) CreateRemote(remoteName, remoteURL string) error {
	_, err := r.executor("remote", "add", remoteName, remoteURL).executeString()
	if err != nil {
		return fmt.Errorf("unable to add remote: %w", err)
	}

	return nil
}

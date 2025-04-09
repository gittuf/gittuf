// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"errors"
	"fmt"
)

var ErrRequiredOptionNotSet = errors.New("required option not set")

type Options struct {
	PrePush *PrePushOptions
}

type Option func(o *Options)

// WithPrePush can be used to specify arguments normally passed to Git pre-push
// hooks.
func WithPrePush(remoteName, remoteURL string, refSpecs []string) Option {
	return func(o *Options) {
		o.PrePush = &PrePushOptions{
			RemoteName: remoteName,
			RemoteURL:  remoteURL,
			RefSpecs:   refSpecs,
		}
	}
}

type PrePushOptions struct {
	RemoteName string
	RemoteURL  string
	RefSpecs   []string
}

func (o *PrePushOptions) Validate() error {
	if o.RemoteName == "" {
		return fmt.Errorf("%w: 'remoteName'", ErrRequiredOptionNotSet)
	}

	if o.RemoteURL == "" {
		return fmt.Errorf("%w: 'remoteURL'", ErrRequiredOptionNotSet)
	}

	if len(o.RefSpecs) == 0 {
		return fmt.Errorf("%w: 'refSpecs'", ErrRequiredOptionNotSet)
	}

	return nil
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package hooks

type Options struct {
	RemoteName string
	RemoteURL  string
	RefSpecs   []string
}

type Option func(o *Options)

// WithPrePush can be used to specify arguments normally passed to Git pre-push
// hooks.
func WithPrePush(remoteName, remoteURL string, refSpecs []string) Option {
	return func(o *Options) {
		o.RemoteName = remoteName
		o.RemoteURL = remoteURL
		o.RefSpecs = refSpecs
	}
}

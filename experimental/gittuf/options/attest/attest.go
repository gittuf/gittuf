// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attest

type Options struct {
	CreateRSLEntry bool
	Expires        string
}

type Option func(o *Options)

func WithRSLEntry() Option {
	return func(o *Options) {
		o.CreateRSLEntry = true
	}
}

func WithExpires(expires string) Option {
	return func(o *Options) {
		o.Expires = expires
	}
}

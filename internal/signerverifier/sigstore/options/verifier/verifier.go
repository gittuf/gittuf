// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

const (
	defaultRekorURL = "https://rekor.sigstore.dev"
)

type Options struct {
	RekorURL string
}

var DefaultOptions = &Options{
	RekorURL: defaultRekorURL,
}

type Option func(o *Options)

func WithRekorURL(rekorURL string) Option {
	return func(o *Options) {
		o.RekorURL = rekorURL
	}
}

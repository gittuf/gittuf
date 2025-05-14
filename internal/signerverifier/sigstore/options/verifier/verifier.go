// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

const (
	defaultRekorURL = "https://rekor.sigstore.dev"
)

type Options struct {
	RekorURL    string
	OfflineMode bool
}

var DefaultOptions = &Options{
	RekorURL:    defaultRekorURL,
	OfflineMode: false,
}

type Option func(o *Options)

func WithRekorURL(rekorURL string) Option {
	return func(o *Options) {
		o.RekorURL = rekorURL
	}
}

func WithOfflineMode(offline bool) Option {
	return func(o *Options) {
		o.OfflineMode = offline
	}
}

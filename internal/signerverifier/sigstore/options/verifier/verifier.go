// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import "github.com/sigstore/sigstore-go/pkg/root"

const (
	defaultRekorURL = "https://rekor.sigstore.dev"
)

type Options struct {
	RekorURL    string
	TrustedRoot root.TrustedMaterial
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

func WithTrustedRoot(trustedRoot root.TrustedMaterial) Option {
	return func(o *Options) {
		o.TrustedRoot = trustedRoot
	}
}

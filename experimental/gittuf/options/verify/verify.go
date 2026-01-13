// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verify

import sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"

type Options struct {
	RefNameOverride string
	LatestOnly      bool

	GranularVSAsPath           string
	MetaVSAPath                string
	SourceProvenanceBundlePath string
	VSASigner                  sslibdsse.SignerVerifier
}

type Option func(o *Options)

func WithOverrideRefName(refNameOverride string) Option {
	return func(o *Options) {
		o.RefNameOverride = refNameOverride
	}
}

func WithLatestOnly() Option {
	return func(o *Options) {
		o.LatestOnly = true
	}
}

func WithGranularVSAsPath(location string) Option {
	return func(o *Options) {
		o.GranularVSAsPath = location
	}
}

func WithMetaVSAPath(location string) Option {
	return func(o *Options) {
		o.MetaVSAPath = location
	}
}

func WithSourceProvenanceBundlePath(location string) Option {
	return func(o *Options) {
		o.SourceProvenanceBundlePath = location
	}
}

func WithVSASigner(signer sslibdsse.SignerVerifier) Option {
	return func(o *Options) {
		o.VSASigner = signer
	}
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package signer

const (
	defaultIssuerURL = "https://oauth2.sigstore.dev/auth"
	defaultClientID  = "sigstore"
	defaultFulcioURL = "https://fulcio.sigstore.dev"
	defaultRekorURL  = "https://rekor.sigstore.dev"
)

type Options struct {
	IssuerURL   string
	ClientID    string
	RedirectURL string
	FulcioURL   string
	RekorURL    string
}

var DefaultOptions = &Options{
	IssuerURL: defaultIssuerURL,
	ClientID:  defaultClientID,
	FulcioURL: defaultFulcioURL,
	RekorURL:  defaultRekorURL,
}

type Option func(o *Options)

func WithIssuerURL(issuerURL string) Option {
	return func(o *Options) {
		o.IssuerURL = issuerURL
	}
}

func WithClientID(clientID string) Option {
	return func(o *Options) {
		o.ClientID = clientID
	}
}

func WithRedirectURL(redirectURL string) Option {
	return func(o *Options) {
		o.RedirectURL = redirectURL
	}
}

func WithFulcioURL(fulcioURL string) Option {
	return func(o *Options) {
		o.FulcioURL = fulcioURL
	}
}

func WithRekorURL(rekorURL string) Option {
	return func(o *Options) {
		o.RekorURL = rekorURL
	}
}

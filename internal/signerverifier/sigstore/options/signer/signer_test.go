// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package signer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	assert.Equal(t, defaultIssuerURL, DefaultOptions.IssuerURL)
	assert.Equal(t, defaultClientID, DefaultOptions.ClientID)
	assert.Equal(t, defaultFulcioURL, DefaultOptions.FulcioURL)
	assert.Equal(t, defaultRekorURL, DefaultOptions.RekorURL)
}

func TestWithIssuerURL(t *testing.T) {
	options := &Options{}

	option := WithIssuerURL("example.com")

	option(options)

	assert.Equal(t, "example.com", options.IssuerURL)
}

func TestWithClientID(t *testing.T) {
	options := &Options{}

	option := WithClientID("abc123xyz")

	option(options)

	assert.Equal(t, "abc123xyz", options.ClientID)
}

func TestWithRedirectURL(t *testing.T) {
	options := &Options{}

	option := WithRedirectURL("example.com")

	option(options)

	assert.Equal(t, "example.com", options.RedirectURL)
}

func TestWithFulcioURL(t *testing.T) {
	options := &Options{}

	option := WithFulcioURL("example.com")

	option(options)

	assert.Equal(t, "example.com", options.FulcioURL)
}

func TestWithRekorURL(t *testing.T) {
	options := &Options{}

	option := WithRekorURL("example.com")

	option(options)

	assert.Equal(t, "example.com", options.RekorURL)
}

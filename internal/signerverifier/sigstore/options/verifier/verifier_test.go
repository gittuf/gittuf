// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	assert.Equal(t, defaultRekorURL, DefaultOptions.RekorURL)
}

func TestWithRekorURL(t *testing.T) {
	opts := &Options{}
	customURL := "https://custom.rekor.example.com"

	option := WithRekorURL(customURL)
	option(opts)

	assert.Equal(t, customURL, opts.RekorURL)
}

func TestWithRekorURL_DoesNotMutateDefaultOptions(t *testing.T) {
	customURL := "https://custom.rekor.example.com"

	option := WithRekorURL(customURL)
	// Apply to a copy, not DefaultOptions
	opts := &Options{RekorURL: DefaultOptions.RekorURL}
	option(opts)

	assert.Equal(t, customURL, opts.RekorURL)
	assert.Equal(t, defaultRekorURL, DefaultOptions.RekorURL)
}

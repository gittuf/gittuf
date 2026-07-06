// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithRekorURL(t *testing.T) {
	options := &Options{}

	option := WithRekorURL("example.com")

	option(options)

	assert.Equal(t, "example.com", options.RekorURL)
}

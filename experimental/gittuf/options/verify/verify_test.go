// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithOverrideRefName(t *testing.T) {
	options := &Options{}

	option := WithOverrideRefName("refs/heads/main")

	option(options)

	assert.Equal(t, "refs/heads/main", options.RefNameOverride)
}

func TestWithLatestOnly(t *testing.T) {
	options := &Options{}

	option := WithLatestOnly()

	option(options)

	assert.True(t, options.LatestOnly)
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithRepositoryLocation(t *testing.T) {
	options := &Options{}

	option := WithRepositoryLocation("example.com")

	option(options)

	assert.Equal(t, "example.com", options.RepositoryLocation)
}

func TestWithRSLEntry(t *testing.T) {
	options := &Options{}

	option := WithRSLEntry()

	option(options)

	assert.True(t, options.CreateRSLEntry)
}

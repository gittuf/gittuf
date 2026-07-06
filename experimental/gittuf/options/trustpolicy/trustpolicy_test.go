// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package trustpolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithRSLEntry(t *testing.T) {
	options := &Options{}

	option := WithRSLEntry()

	option(options)

	assert.True(t, options.CreateRSLEntry)
}

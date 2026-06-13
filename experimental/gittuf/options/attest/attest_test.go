// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRSLOptions(t *testing.T) {
	options := &Options{}

	option := WithRSLEntry()
	option(options)

	assert.True(t, options.CreateRSLEntry)
}

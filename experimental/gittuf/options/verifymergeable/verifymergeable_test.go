// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifymergeable

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithBypassRSLForFeatureRef(t *testing.T) {
	options := &Options{}

	option := WithBypassRSLForFeatureRef()

	option(options)

	assert.True(t, options.BypassRSLForFeatureRef)
}

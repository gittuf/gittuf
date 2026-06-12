// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package luasandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithLuaTimeout(t *testing.T) {
	options := &EnvironmentOptions{}

	option := WithLuaTimeout(10)

	option(options)

	assert.Equal(t, 10, options.LuaTimeout)
}

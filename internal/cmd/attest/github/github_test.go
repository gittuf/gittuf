// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"testing"

	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	options := &persistent.Options{}
	cmd := New(options)
	assert.Equal(t, "github", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.True(t, cmd.HasSubCommands())
}

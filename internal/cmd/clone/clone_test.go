// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package clone

import (
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/stretchr/testify/assert"
)

func TestClone(t *testing.T) {
	t.Run("no arguments", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "accepts between 1 and 2 arg(s)")
	})

	t.Run("invalid root key", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New(), "/non/existent/path", "--root-key", "/non/existent/key")
		assert.ErrorContains(t, err, "failed to run command")
	})
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package luasandbox

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestNewLuaEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(t.Context(), repo)
	assert.Nil(t, err)
	assert.NotNil(t, environment)
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package luasandbox

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/stretchr/testify/assert"
)

var (
	testCtx = context.Background()
)

func TestNewLuaEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	environment, err := NewLuaEnvironment(testCtx, repo)
	assert.Nil(t, err)
	assert.NotNil(t, environment)
	environment.Cleanup()
}

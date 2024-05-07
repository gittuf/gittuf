// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir)

	// CreateTestGitRepository sets our test config
	config, err := repo.GetGitConfig()
	assert.Nil(t, err)
	assert.Equal(t, testName, config["user.name"])
	assert.Equal(t, testEmail, config["user.email"])
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithPrePush(t *testing.T) {
	options := &Options{}

	option := WithPrePush("origin", "example.com", []string{"refs/heads/main:refs/heads/main"})

	option(options)

	assert.Equal(t, "origin", options.PrePush.RemoteName)
	assert.Equal(t, "example.com", options.PrePush.RemoteURL)
	assert.Equal(t, []string{"refs/heads/main:refs/heads/main"}, options.PrePush.RefSpecs)
}

func TestPrePushOptionsValidate(t *testing.T) {
	options := &PrePushOptions{
		RemoteName: "",
	}

	err := options.Validate()
	assert.ErrorContains(t, err, "remoteName")

	options.RemoteName = "origin"

	err = options.Validate()
	assert.ErrorContains(t, err, "remoteURL")

	options.RemoteURL = "example.com"

	err = options.Validate()
	assert.ErrorContains(t, err, "refSpecs")

	options.RefSpecs = []string{"refs/heads/main:refs/heads/main"}

	err = options.Validate()
	assert.Nil(t, err)
}

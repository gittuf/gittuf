// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestLoadStateOptions(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		opts := &LoadStateOptions{}
		assert.Empty(t, opts.InitialRootPrincipals)
		assert.False(t, opts.BypassRSL)
	})

	t.Run("WithInitialRootPrincipals", func(t *testing.T) {
		principals := []tuf.Principal{}
		opt := WithInitialRootPrincipals(principals)

		opts := &LoadStateOptions{}
		opt(opts)

		assert.Equal(t, principals, opts.InitialRootPrincipals)
		assert.False(t, opts.BypassRSL)
	})

	t.Run("BypassRSL", func(t *testing.T) {
		opt := BypassRSL()

		opts := &LoadStateOptions{}
		opt(opts)

		assert.True(t, opts.BypassRSL)
		assert.Empty(t, opts.InitialRootPrincipals)
	})

	t.Run("multiple options", func(t *testing.T) {
		principals := []tuf.Principal{}
		opts := &LoadStateOptions{}

		WithInitialRootPrincipals(principals)(opts)
		BypassRSL()(opts)

		assert.Equal(t, principals, opts.InitialRootPrincipals)
		assert.True(t, opts.BypassRSL)
	})
}

// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicKeys(t *testing.T) {
	t.Parallel()

	pk := &PublicKeys{}

	// Test Type()
	assert.Equal(t, "public-keys", pk.Type())

	// Test Set() and String() with initial value
	err := pk.Set("key1.pub")
	require.NoError(t, err)
	assert.Equal(t, "key1.pub", pk.String())

	// Test Set() appending a second value and String() formatting with comma separation
	err = pk.Set("key2.pub")
	require.NoError(t, err)
	assert.Equal(t, "key1.pub, key2.pub", pk.String())
}

func TestSubcommandRequired(t *testing.T) {
	cmd := &cobra.Command{Use: "group"}

	assert.ErrorIs(t, SubcommandRequired(cmd, nil), pflag.ErrHelp)
	assert.ErrorContains(t, SubcommandRequired(cmd, []string{"nosuch"}), `unknown command "nosuch" for "group"`)
}

func TestShowHelp(t *testing.T) {
	cmd := &cobra.Command{Use: "group", Long: "group help"}
	out := &bytes.Buffer{}
	cmd.SetOut(out)

	assert.NoError(t, ShowHelp(cmd, nil))
	assert.Contains(t, out.String(), "group help")
}

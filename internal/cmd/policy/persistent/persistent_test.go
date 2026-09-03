// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package persistent

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestAddPersistentFlags(t *testing.T) {
	o := &Options{}
	cmd := &cobra.Command{Use: "test"}
	o.AddPersistentFlags(cmd)

	assert.NotNil(t, cmd.PersistentFlags().Lookup("signing-key"))
	assert.NotNil(t, cmd.PersistentFlags().ShorthandLookup("k"))
	assert.NotNil(t, cmd.PersistentFlags().Lookup("create-rsl-entry"))

	err := cmd.ParseFlags([]string{"-k", "test-key", "--create-rsl-entry"})
	assert.Nil(t, err)
	assert.Equal(t, "test-key", o.SigningKey)
	assert.True(t, o.WithRSLEntry)
}

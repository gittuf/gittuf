// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"

	"github.com/spf13/cobra"
)

// Borrowed from https://github.com/spf13/cobra/blob/main/command_test.go as
// suggested in https://opdev.github.io/cobra-primer/hands_on/testing.html.
func ExecuteCommandC(root *cobra.Command, args ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer, error) {
	stdOut, stdErr := new(bytes.Buffer), new(bytes.Buffer)
	root.SetOut(stdOut)
	root.SetErr(stdErr)
	root.SetArgs(args)

	c, err := root.ExecuteC()

	return c, stdOut, stdErr, err
}

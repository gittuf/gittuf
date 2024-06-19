// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os/exec"
	"errors"
	"github.com/spf13/cobra"
)

// gitCmd represents the base command when called without any subcommands
var gitCmd = &cobra.Command{
	Use:               "gittuf-git", // This sets "git" as the required subcommand
	Short:             "Run git commands with potential gittuf integration",
	Args:              cobra.ArbitraryArgs, 
	RunE:              runGitCommand,
	DisableAutoGenTag: true,
}

func runGitCommand(cmd *cobra.Command, args []string) error {
	// Construct the full git command, including 'git'
	gitArgs := append([]string{"git"}, args...)

	// Execute the git command
	gitCmd := exec.Command(gitArgs[0], gitArgs[1:]...) //nolint:gosec
	gitCmd.Stdout = cmd.OutOrStdout()
	gitCmd.Stderr = cmd.ErrOrStderr()

	err := gitCmd.Run()

	if err != nil {
		// Custom error handling (same as before)
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr := string(ee.Stderr)
			return fmt.Errorf("%s", stderr)
		}
		return fmt.Errorf("git command failed: %w", err)
	}

	return nil
}

func New() *cobra.Command {
	// Disable flag parsing for 'gittuf git' so it doesn't consume --help
	gitCmd.DisableFlagParsing = true  

	return gitCmd
}

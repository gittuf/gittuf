// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	// Check if enough arguments are provided
	if len(os.Args) < 2 {
		fmt.Println("Usage: gittuf-git <git-command> [<args>]")
		gitCmd := exec.Command("git", "--help")
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		err := gitCmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running git help command: %v\n", err)
		}
		os.Exit(1)
	}

	gitArgs := os.Args[1:]
	gitCommand := gitArgs[0]

	switch gitCommand {
	case "push":
		gittufPush(gitArgs)
	default:
		// this code runs when a command that doesn't need additional GitTuf features is executed.
		fmt.Print("Default Section")
	}

	gitCmd := exec.Command("git", gitArgs...)
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr

	err := gitCmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			fmt.Fprintln(os.Stderr, string(exitErr.Stderr)) // Output to stderr
		} else {
			fmt.Fprintf(os.Stderr, "Error running git command: %v\n", err)
		}
		os.Exit(1)
	}
}

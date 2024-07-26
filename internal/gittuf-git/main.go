// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/gittuf/gittuf/internal/gittuf-git/push"
)

func main() {
	gitArgs := os.Args[1:]

	switch determineGitOp(gitArgs) {
	case "push":
		push.GittufPush(gitArgs)
	case "other":
		gitCmd := exec.Command("git", gitArgs...)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr

		err := gitCmd.Run()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				fmt.Fprintln(os.Stderr, string(exitErr.Stderr))
			} else {
				fmt.Fprintf(os.Stderr, "Error running git command: %v\n", err)
			}
			os.Exit(1)
		}
	}
}

func determineGitOp(args []string) string {
	for _, a := range args {
		if a == "push" {
			return "push"
		} else if a == "pull" {
			return "pull"
		}
	}
	return "other"
}

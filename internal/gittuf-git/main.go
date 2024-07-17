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

	if checkArg("push", gitArgs){
		push.GittufPush(gitArgs)
	} else {
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

func checkArg(arg string, args []string) bool{
	for _, a := range args {
		if a == arg {
			return true
			}
	}
	return false
}

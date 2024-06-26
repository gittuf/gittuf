// SPDX-License-Identifier: Apache-2.0

package push

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/internal/cmd/rsl/record"
)

func splitPushArgs(args []string) (rslRecordArgs []string) {
	rslRecordArgs = []string{}

	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			if arg != "origin" {
				rslRecordArgs = append(rslRecordArgs, arg)
			}
		}
	}
	return rslRecordArgs
}

func GittufPush(gitArgs []string) {
	rslRecordArgs := splitPushArgs(gitArgs[1:])
	recordCmd := record.New()
	rslArg := []string{}
	for _, arg := range rslRecordArgs {
		rslArg = append(rslArg, arg)
		if err := recordCmd.RunE(nil, rslArg); err != nil {
			fmt.Fprintf(os.Stderr, "failed to record RSL entry: %v\n", err)
		}
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

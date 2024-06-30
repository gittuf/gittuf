// SPDX-License-Identifier: Apache-2.0

package push

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/internal/cmd/rsl/record"
	"github.com/gittuf/gittuf/internal/cmd/rsl/skiprewritten"
)

// splitPushArgs splits the push arguments and filters out flags and "origin".
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

// GittufPush handles the gittuf push operation.
func GittufPush(gitArgs []string) {
	rslRecordArgs := splitPushArgs(gitArgs[1:])
	recordCmd := record.New()
	skiprewrittenCmd := skiprewritten.New()
	rslArg := []string{}
	for _, arg := range rslRecordArgs {
		rslArg = append(rslArg, arg)
		if err := skiprewrittenCmd.RunE(nil, rslArg); err != nil {
			fmt.Fprintf(os.Stderr, "failed to run skip-rewritten command: %v\n", err)
		}
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

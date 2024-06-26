package main

import (
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

func gittufPush(gitArgs []string) {
	rslRecordArgs := splitPushArgs(gitArgs[1:])
	recordCmd := record.New()
	rslArg := []string{}
	for _, arg := range rslRecordArgs {
		rslArg = append(rslArg, arg)
		hasRebased, err := hasRefBeenRebased(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to check for rebase: %v", err)
		}
		if hasRebased {
			rslArg = append(rslArg, "--skip-rewritten")
		}
		// rsl record command
		if err := recordCmd.RunE(nil, rslArg); err != nil {
			fmt.Fprintf(os.Stderr, "failed to record RSL entry: %v\n", err)
		}
	}
}

func hasRefBeenRebased(ref string) (bool, error) {
	cmd := exec.Command("git", "log", "--left-right", "--cherry-pick", "origin/"+ref+"..."+ref) // #nosec
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check for rebase: %w", err)
	}
	// If there is output, it means there are unique commits on the local branch that haven't
	// been merged to the remote, indicating a rebase

	return len(output) > 0, nil
}

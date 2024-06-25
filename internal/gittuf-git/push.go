package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/gittuf/gittuf/internal/cmd/rsl"
)

func splitPushArgs(args []string) (rslRecordArgs []string) {
	rslRecordArgs = []string{}

	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			rslRecordArgs = append(rslRecordArgs, arg)
		}
	}
	return rslRecordArgs
}

func gittufPush(gitArgs []string) {
	rslRecordArgs := splitPushArgs(gitArgs[1:])
	recordCmd := rsl.New()
	rslArg := []string{}
	for _, arg := range rslRecordArgs {
		rslArg = append(rslArg, arg)
		if err := recordCmd.RunE(nil, rslArg); err != nil {
			fmt.Fprintf(os.Stderr, "failed to record RSL entry: %v\n", err)
		}
	}
}

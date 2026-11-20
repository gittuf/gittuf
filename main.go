// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/gittuf/gittuf/internal/cmd/profile"
	"github.com/gittuf/gittuf/internal/cmd/root"
	"github.com/gittuf/gittuf/internal/version"
)

func main() {
	os.Exit(run())
}

// run executes the gittuf command and returns the exit code. The cleanup is
// deferred here rather than in main so that it also runs when the command
// fails, since deferred functions do not run under os.Exit.
func run() (exitCode int) {
	defer func() {
		if err := profile.StopProfiling(); err != nil {
			fmt.Fprintf(os.Stderr, "unexpected profiling error: %s\n", err.Error())
		}

		root.ReportStorerTrace(os.Stderr)

		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "unexpected error: %s\n\n", fmt.Sprint(r))
			debug.PrintStack()
			fmt.Fprintln(os.Stderr, "\nPlease consider filing a bug on https://github.com/gittuf/gittuf/issues with the stack trace and steps to reproduce this state. Thanks!")

			exitCode = 1
		}
	}()

	rootCmd := root.New()
	if err := rootCmd.Execute(); err != nil {
		// Report the storer trace before the upgrade guidance so that the
		// guidance is the last thing printed. It emits at most once.
		root.ReportStorerTrace(os.Stderr)

		if root.IsUpgradeRequiredError(err) {
			fmt.Fprintf(os.Stderr, "\n%s\nThis client is gittuf %s.\n", root.UpgradeGuidance, version.GetVersion())
		}

		return 1
	}

	return 0
}

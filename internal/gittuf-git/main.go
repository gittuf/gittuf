// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/gittuf/gittuf/internal/gittuf-git/args"
	"github.com/gittuf/gittuf/internal/gittuf-git/cmd"
)

/*
	In addition to the git-remote-gittuf (transport) and the gittuf binary,
	there is another way to use gittuf in a day-to-day workflow: as a drop-in
	replacement for the git binary.

	The gittuf-git command compatibility binary intercepts "interesting"
	operations, such as pushing and pulling, performs gittuf-specific steps, and
	then hands execution over to the actual Git binary.

	All other operations which do not necessitate the creation or manipulation
	of gittuf metadata are directly passed onto the Git binary on the system
	unmodified.
*/

func main() {
	// The main flow is simple:
	// 1.  Process the arguments to facilitate step 2.
	// 2.  Identify the Git operation.
	// 3a. If the operation is something that we'll want to invoke gittuf for,
	// 	   then do what we need with gittuf and then invoke Git as appropriate.
	// 3b. Otherwise, simply pass all arguments to Git.

	// Process args
	var rawArgs []string
	var gitArgs args.Args

	if len(os.Args) < 2 { // No arguments to git
		gitArgs = args.Args{
			Command: "",
		}
	} else {
		// Trim off the first argument; we don't need it.
		rawArgs = os.Args[1:]
		gitArgs = args.ProcessArgs(rawArgs)
	}

	// Categorize the Git operation.
	switch {
	case gitArgs.Command == "clone":
		err := cmd.Clone(gitArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	case gitArgs.Command == "push":
		err := cmd.Push(gitArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	case (gitArgs.Command == "pull") || (gitArgs.Command == "fetch"):
		err := cmd.PullOrFetch(gitArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	default:
		// If the git operation isn't one of the above ones, just send the args
		// over to git without any gittuf invocation
		gitCmd := exec.Command("git", rawArgs...)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr

		err := gitCmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	}
}

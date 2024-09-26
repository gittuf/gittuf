// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/internal/gittuf-git/args"
	"github.com/gittuf/gittuf/internal/repository"
	rslopts "github.com/gittuf/gittuf/internal/repository/options/rsl"
	"github.com/gittuf/gittuf/internal/tuf"
)

// Clone handles the clone operation for gittuf + git
func Clone(gitArgs args.Args) error {
	// Set working directory as needed
	if gitArgs.ChdirIdx > 0 {
		if err := os.Chdir(gitArgs.GlobalFlags[gitArgs.ChdirIdx]); err != nil {
			return err
		}
	}

	// Clone the repository using gittuf
	var dir string
	if len(gitArgs.Parameters) > 2 {
		dir = gitArgs.Parameters[1]
	} else {
		dir = ""
	}

	_, err := repository.Clone(context.Background(), gitArgs.Parameters[0], dir, "", []*tuf.Key{})
	return err
}

// PullOrFetch handles the pull or fetch operation for gittuf + git
func PullOrFetch(gitArgs args.Args) error {
	// Set working directory as needed
	if gitArgs.ChdirIdx > 0 {
		if err := os.Chdir(gitArgs.GlobalFlags[gitArgs.ChdirIdx]); err != nil {
			return err
		}
	}

	// Pull non-RSL changes
	cmdArgs := []string{gitArgs.Command}
	cmdArgs = append(cmdArgs, gitArgs.Parameters...)
	// TODO: Make this more robust in case of symlinks to git
	gitPullCmd := exec.Command("git", cmdArgs...)
	gitPullCmd.Stdout = os.Stdout
	gitPullCmd.Stderr = os.Stderr

	if err := gitPullCmd.Run(); err != nil {
		return err
	}

	// Pull RSL changes
	remote := args.DetermineRemote(gitArgs.Parameters)
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	if err := repo.PullRSL(remote); err != nil {
		return err
	}

	// Pull policy changes
	return repo.PullPolicy(remote)
}

// Push handles the push operation for gittuf + git
func Push(gitArgs args.Args) error {
	// Set working directory as needed
	if gitArgs.ChdirIdx > 0 {
		if err := os.Chdir(gitArgs.GlobalFlags[gitArgs.ChdirIdx]); err != nil {
			return err
		}
	}

	// Record changes to the RSL
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	var refName string
	if len(gitArgs.Parameters) > 1 {
		refParts := strings.Split(gitArgs.Parameters[1], ":")
		if len(refParts) > 0 {
			refName = refParts[0]
		} else {
			refName = gitArgs.Parameters[1]
		}
	} else {
		refName = "HEAD"
	}

	// TODO: This needs to record the appropriate reference being pushed.
	if err := repo.RecordRSLEntryForReference(refName, true, rslopts.WithOverrideRefName(refName)); err != nil {
		return err
	}

	// Push non-RSL changes to the remote
	cmdArgs := []string{gitArgs.Command}
	cmdArgs = append(cmdArgs, gitArgs.Parameters...)
	// TODO: Make this more robust in case of symlinks to git
	gitPushCmd := exec.Command("git", cmdArgs...)
	gitPushCmd.Stdout = os.Stdout
	gitPushCmd.Stderr = os.Stderr

	if err := gitPushCmd.Run(); err != nil {
		return err
	}

	// Push RSL changes to the remote
	remote := args.DetermineRemote(gitArgs.Parameters)

	if err := repo.PushRSL(remote); err != nil {
		return err
	}

	if err := repo.PushPolicy(remote); err != nil {
		return err
	}

	return nil
}

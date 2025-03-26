// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/gittuf-git/args"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
)

// Clone handles the clone operation for gittuf + git
func Clone(ctx context.Context, gitArgs args.Args) error {
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

	_, err := gittuf.Clone(ctx, gitArgs.Parameters[0], dir, "", nil, false)
	return err
}

// SyncWithRemote handles the pull, fetch and push operations for gittuf + git
func SyncWithRemote(ctx context.Context, gitArgs args.Args) error {
	// Set working directory as needed
	if gitArgs.ChdirIdx > 0 {
		if err := os.Chdir(gitArgs.GlobalFlags[gitArgs.ChdirIdx]); err != nil {
			return err
		}
	}

	if gitArgs.Command == "push" {
		// Record changes to RSL
		repo, err := gittuf.LoadRepository()
		if err != nil {
			return err
		}

		refName := determineRef(gitArgs)

		if err := repo.RecordRSLEntryForReference(ctx, refName, true, rslopts.WithOverrideRefName(refName)); err != nil {
			return err
		}
	}

	// Sync non-RSL changes (user specified command)
	cmdArgs := []string{gitArgs.Command}
	cmdArgs = append(cmdArgs, gitArgs.Parameters...)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	gitSyncCmd := exec.Command(gitPath, cmdArgs...)
	gitSyncCmd.Stdout = os.Stdout
	gitSyncCmd.Stderr = os.Stderr

	if err := gitSyncCmd.Run(); err != nil {
		return err
	}

	if gitArgs.Command == "pull" || gitArgs.Command == "fetch" {
		fetchCmdArgs := []string{"fetch", "--atomic"}

		// Add remote if required by pull command with simple git pull
		if gitArgs.Command == "pull" && len(gitArgs.Parameters) == 0 {
			gitConfig, err := args.GetGitConfig(".git")
			if err != nil {
				fmt.Println("Error while retrieving git config")
				return err
			}
			remote := gitConfig["branch.main.remote"]
			fetchCmdArgs = append(fetchCmdArgs, remote)
		}

		fetchCmdArgs = append(fetchCmdArgs,
			"refs/gittuf/reference-state-log:refs/gittuf/reference-state-log",
			"refs/gittuf/policy:refs/gittuf/policy")

		gitFetchCmd := exec.Command(gitPath, fetchCmdArgs...)
		gitFetchCmd.Stdout = os.Stdout
		gitFetchCmd.Stderr = os.Stderr

		return gitFetchCmd.Run()
	}

	return nil
}

// Commit handles the commit operation for gittuf + git
func Commit(ctx context.Context, gitArgs args.Args) error { //nolint:revive
	if gitArgs.ChdirIdx > 0 {
		if err := os.Chdir(gitArgs.GlobalFlags[gitArgs.ChdirIdx]); err != nil {
			return err
		}
	}

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	// TODO: Do we attempt to verify here every time?

	// TODO: Change this maybe?
	principalID := os.Getenv("GITTUF_GIT_PRINCIPAL_ID")

	// Invoke pre-commit hook
	exitCodes, err := repo.InvokeHook(context.Background(), tuf.HookStagePreCommit, principalID, policy.TargetsRoleName)
	if err != nil {
		return err
	}

	// Check if any of the exit codes are non-zero
	for hookName, exitCode := range exitCodes {
		if exitCode != 0 {
			return fmt.Errorf("pre-commit hook %s failed with exit code %d", hookName, exitCode)
		}
	}

	// Commit irrespective of failed verification. However, verification is
	// important for debugging purposes. The user should be able to keep
	// track of whether and why verification is failing.
	cmdArgs := []string{gitArgs.Command}
	cmdArgs = append(cmdArgs, gitArgs.Parameters...)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return err
	}

	gitPushCmd := exec.Command(gitPath, cmdArgs...)
	gitPushCmd.Stdout = os.Stdout
	gitPushCmd.Stderr = os.Stderr

	if err := gitPushCmd.Run(); err != nil {
		return err
	}

	return nil
}

// determineRef parses the git reference from the given command-line arguments.
//
// Parameters:
//   gitArgs (args.Args): Struct containing the command-line arguments passed to the Git command.
//
// Returns:
//   string: The Git reference name or "HEAD" if no reference is provided.

func determineRef(gitArgs args.Args) string {
	var refName string
	if len(gitArgs.Parameters) > 1 {
		refParts := strings.Split(gitArgs.Parameters[1], ":")
		if len(refParts) > 0 {
			for i := range refParts {
				if !strings.HasPrefix(refParts[i], "-") {
					refName = refParts[0]
					break
				}
			}
		} else {
			refName = gitArgs.Parameters[1]
		}
	} else {
		refName = "HEAD"
	}
	return refName
}

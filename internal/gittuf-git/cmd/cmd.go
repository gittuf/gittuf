// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/gittuf/gittuf/experimental/gittuf"
	hookopts "github.com/gittuf/gittuf/experimental/gittuf/options/hooks"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/gittuf-git/args"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
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

	gittufRepo, err := gittuf.LoadRepository(gitArgs.RootDir)
	if err != nil {
		return err
	}

	gitinterfaceRepo, err := gitinterface.LoadRepository(gitArgs.RootDir)
	if err != nil {
		return err
	}

	if gitArgs.Command == "push" {
		// Record changes to RSL and invoke pre-push hooks
		pushedRefs, err := args.DeterminePushedRefs(gitinterfaceRepo, gitArgs)
		if err != nil {
			return err
		}

		for _, ref := range pushedRefs {
			if err := gittufRepo.RecordRSLEntryForReference(ctx, ref, true, rslopts.WithOverrideRefName(ref)); err != nil {
				return err
			}
		}

		remoteName, remoteURL, err := args.DetermineRemote(gitArgs)
		if err != nil {
			return err
		}

		// This is left in for debugging purposes
		var signer dsse.Signer
		if os.Getenv("GITTUF_DEV") == "1" && os.Getenv("GITTUF_GIT_SSH_KEYPATH") != "" {
			signer, err = loadDebugSigner(os.Getenv("GITTUF_GIT_SSH_KEYPATH"))
			if err != nil {
				return err
			}
		} else {
			// If the signer is nil, then gittuf will attempt to load a signer from
			// the user's Git configuration
			signer = nil
		}

		pushOpts := hookopts.WithPrePush(remoteName, remoteURL, pushedRefs)

		repo, err := gittuf.LoadRepository(gitArgs.RootDir)
		if err != nil {
			return err
		}

		_, err = repo.InvokeHooksForStage(ctx, signer, tuf.HookStagePrePush, pushOpts)
		if err != nil {
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
			config, err := gitinterfaceRepo.GetGitConfig()
			if err != nil {
				fmt.Println("error while retrieving git config")
				return err
			}
			remote := config["branch.main.remote"]
			fetchCmdArgs = append(fetchCmdArgs, remote)

			fetchCmdArgs = append(fetchCmdArgs,
				"refs/gittuf/reference-state-log:refs/gittuf/reference-state-log",
				"refs/gittuf/policy:refs/gittuf/policy",
				"refs/gittuf/policy-staging:refs/gittuf/policy-staging",
				"refs/gittuf/attestations:refs/gittuf/attestations")
		}

		gitFetchCmd := exec.Command(gitPath, fetchCmdArgs...)
		gitFetchCmd.Stdout = os.Stdout
		gitFetchCmd.Stderr = os.Stderr

		return gitFetchCmd.Run()
	}

	return nil
}

// Commit handles the commit operation for gittuf + git
func Commit(_ context.Context, gitArgs args.Args) error { //nolint:revive
	if gitArgs.ChdirIdx > 0 {
		if err := os.Chdir(gitArgs.GlobalFlags[gitArgs.ChdirIdx]); err != nil {
			return err
		}
	}

	repo, err := gittuf.LoadRepository(gitArgs.RootDir)
	if err != nil {
		return err
	}

	// TODO: Do we attempt to verify here every time?

	// This is left in for debugging purposes
	var signer dsse.Signer
	if os.Getenv("GITTUF_DEV") == "1" && os.Getenv("GITTUF_GIT_SSH_KEYPATH") != "" {
		signer, err = loadDebugSigner(os.Getenv("GITTUF_GIT_SSH_KEYPATH"))
		if err != nil {
			return err
		}
	} else {
		// If the signer is nil, then gittuf will attempt to load a signer from
		// the user's Git configuration
		signer = nil
	}

	// Invoke pre-commit hook
	exitCodes, err := repo.InvokeHooksForStage(context.Background(), signer, tuf.HookStagePreCommit)
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

// loadDebugSigner attempts to load a signer from the SSH key specified by
// the environment variable GITTUF_GIT_SSH_KEYPATH
func loadDebugSigner(keypath string) (signer dsse.Signer, err error) {
	return ssh.NewSignerFromFile(keypath)
}

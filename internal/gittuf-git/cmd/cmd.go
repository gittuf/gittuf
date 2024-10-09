// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
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
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return err
	}

	gitPullCmd := exec.Command(gitPath, cmdArgs...)
	gitPullCmd.Stdout = os.Stdout
	gitPullCmd.Stderr = os.Stderr

	if err := gitPullCmd.Run(); err != nil {
		return err
	}

	RSLcmdArgs := []string{gitArgs.Command, gitArgs.Parameters[0]}
	RSLcmdArgs = append(RSLcmdArgs, "refs/gittuf/reference-state-log")
	gitPullRSLCmd := exec.Command("git", RSLcmdArgs...)
	gitPullRSLCmd.Stdout = os.Stdout
	gitPullRSLCmd.Stderr = os.Stderr

	if err := gitPullRSLCmd.Run(); err != nil {
		return err
	}

	policyCmdArgs := []string{gitArgs.Command, gitArgs.Parameters[0]}
	policyCmdArgs = append(policyCmdArgs, "refs/gittuf/policy")
	gitPullPolicyCmd := exec.Command("git", policyCmdArgs...)
	gitPullPolicyCmd.Stdout = os.Stdout
	gitPullPolicyCmd.Stderr = os.Stderr

	// Pull policy changes
	return gitPullRSLCmd.Run()
}

// Commit handles the commit operation for gittuf + git
func Commit(gitArgs args.Args) error {
	if gitArgs.ChdirIdx > 0 {
		if err := os.Chdir(gitArgs.GlobalFlags[gitArgs.ChdirIdx]); err != nil {
			return err
		}
	}

	// verify policy
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

	if err = repo.VerifyRef(context.Background(), refName); err != nil {
		fmt.Println("Verification unsuccessful with error: ", err)
	} else {
		fmt.Println("Verification success")
	}

	// Commit irrespective of failed verification. However, verification is
	// important for debugging purposes. The user should be able to keep
	// track of whether and why verification is failing.
	cmdArgs := []string{gitArgs.Command}
	cmdArgs = append(cmdArgs, gitArgs.Parameters...)
	// TODO: Make this more robust in case of symlinks to git
	gitPushCmd := exec.Command("git", cmdArgs...)
	gitPushCmd.Stdout = os.Stdout
	gitPushCmd.Stderr = os.Stderr

	if err := gitPushCmd.Run(); err != nil {
		return err
	}

	return nil
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

	// Push RSL changes to the remote
	remote, err := args.DetermineRemote(gitArgs.Parameters, gitArgs.GitDir)
	if err != nil {
		return err
	}

	if err := repo.PushRSL(remote); err != nil {
		return err
	}

	if err := repo.PushPolicy(remote); err != nil {
		return err
	}

	return nil
}

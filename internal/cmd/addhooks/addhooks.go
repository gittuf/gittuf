// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhooks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
	force     bool
	hookTypes []string
	listHooks bool
	remove    bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(
		&o.force,
		"force",
		"f",
		false,
		"overwrite hooks, if they already exist",
	)

	cmd.Flags().StringSliceVar(
		&o.hookTypes,
		"hooks",
		[]string{"pre-push"},
		"comma-separated list of hook types to install (pre-push, pre-commit, post-commit)",
	)

	cmd.Flags().BoolVar(
		&o.listHooks,
		"list",
		false,
		"list installed gittuf hooks",
	)

	cmd.Flags().BoolVar(
		&o.remove,
		"remove",
		false,
		"remove installed gittuf hooks",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	if o.listHooks {
		return o.listInstalledHooks(cmd, repo)
	}

	if o.remove {
		return o.removeHooks(cmd, repo)
	}

	return o.installHooks(cmd, repo)
}

func (o *options) installHooks(cmd *cobra.Command, repo *gittuf.Repository) error {
	var errorMsgs []string

	for _, hookType := range o.hookTypes {
		hookType = strings.TrimSpace(hookType)

		var script []byte
		var gittufHookType gittuf.HookType

		switch hookType {
		case "pre-push":
			script = generateCrossPlatformScript(hookType, string(prePushScript))
			gittufHookType = gittuf.HookPrePush
		case "pre-commit":
			script = generateCrossPlatformScript(hookType, string(preCommitScript))
			gittufHookType = gittuf.HookPreCommit
		case "post-commit":
			script = generateCrossPlatformScript(hookType, string(postCommitScript))
			gittufHookType = gittuf.HookPostCommit
		default:
			errorMsgs = append(errorMsgs, fmt.Sprintf("unsupported hook type: %s", hookType))
			continue
		}

		err := repo.UpdateGitHook(gittufHookType, script, o.force)
		if err != nil {
			var hookErr *gittuf.ErrHookExists
			if !o.force && errors.As(err, &hookErr) {
				fmt.Fprintf(
					cmd.ErrOrStderr(),
					"'%s' already exists. Use --force flag or merge existing hook and the following script manually:\n\n%s\n",
					string(hookErr.HookType),
					string(script),
				)
				errorMsgs = append(errorMsgs, fmt.Sprintf("hook '%s' already exists", hookType))
			} else {
				errorMsgs = append(errorMsgs, fmt.Sprintf("failed to install %s hook: %v", hookType, err))
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully installed %s hook\n", hookType)
		}
	}

	if len(errorMsgs) > 0 {
		return fmt.Errorf("hook installation errors: %s", strings.Join(errorMsgs, "; "))
	}

	return nil
}

func (o *options) listInstalledHooks(cmd *cobra.Command, repo *gittuf.Repository) error {
	hookTypes := []string{"pre-push", "pre-commit", "post-commit"}

	fmt.Fprintf(cmd.OutOrStdout(), "Installed gittuf hooks:\n")

	gitRepo := repo.GetGitRepository()
	gitDir := gitRepo.GetGitDir()

	for _, hookType := range hookTypes {
		hookPath := filepath.Join(gitDir, "hooks", hookType)
		if _, err := os.Stat(hookPath); err == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", hookType)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s\n", hookType)
		}
	}

	return nil
}

func (o *options) removeHooks(cmd *cobra.Command, repo *gittuf.Repository) error {
	var errorMsgs []string

	// If no specific hook types are provided, remove all supported hooks
	hookTypesToRemove := o.hookTypes
	if len(hookTypesToRemove) == 0 {
		hookTypesToRemove = []string{"pre-push", "pre-commit", "post-commit"}
	}

	gitRepo := repo.GetGitRepository()
	gitDir := gitRepo.GetGitDir()

	for _, hookType := range hookTypesToRemove {
		hookType = strings.TrimSpace(hookType)

		// Validate hook type
		switch hookType {
		case "pre-push", "pre-commit", "post-commit":
			// Valid hook types
		default:
			errorMsgs = append(errorMsgs, fmt.Sprintf("unsupported hook type: %s", hookType))
			continue
		}

		hookFile := filepath.Join(gitDir, "hooks", hookType)

		// Check if hook exists
		if _, err := os.Stat(hookFile); os.IsNotExist(err) {
			fmt.Fprintf(cmd.OutOrStdout(), "Hook %s does not exist, skipping\n", hookType)
			continue
		}

		// Read the hook file to check if it's a gittuf hook
		content, err := os.ReadFile(hookFile)
		if err != nil {
			errorMsgs = append(errorMsgs, fmt.Sprintf("failed to read %s hook: %v", hookType, err))
			continue
		}

		// Only remove if it's a gittuf hook (contains "gittuf" in the content)
		if !strings.Contains(string(content), "gittuf") {
			fmt.Fprintf(cmd.OutOrStdout(), "Hook %s is not a gittuf hook, skipping removal\n", hookType)
			continue
		}

		// Remove the hook file
		err = os.Remove(hookFile)
		if err != nil {
			errorMsgs = append(errorMsgs, fmt.Sprintf("failed to remove %s hook: %v", hookType, err))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully removed %s hook\n", hookType)
		}
	}

	if len(errorMsgs) > 0 {
		return fmt.Errorf("hook removal errors: %s", strings.Join(errorMsgs, "; "))
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "add-hooks",
		Short: "Add git hooks that automatically create and sync RSL",
		Long: `The 'add-hooks' command installs Git hooks that automatically create and sync the RSL when certain Git actions occur, such as a push. By default, it prevents overwriting existing hooks unless the '--force' flag is specified.

Supported hook types:
  - pre-push: Automatically creates RSL entries and syncs with remote before pushing
  - pre-commit: Validates staged changes against gittuf policies  
  - post-commit: Provides guidance on RSL management after commits

Examples:
  gittuf add-hooks                           # Install default pre-push hook
  gittuf add-hooks --hooks pre-push,pre-commit  # Install multiple hooks
  gittuf add-hooks --list                    # List installed hooks
  gittuf add-hooks --remove                  # Remove all gittuf hooks
  gittuf add-hooks --remove --hooks pre-push # Remove specific hook
  gittuf add-hooks --force                   # Force overwrite existing hooks`,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}

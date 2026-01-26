// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/spf13/cobra"
)

type options struct {
	githubToken string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.githubToken,
		"github-token",
		"",
		"GitHub token for enabling gittuf GitHub approvals",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	gitRepo := repo.GetGitRepository()

	gitinterfaceRepo, err := gitinterface.LoadRepository(".")
	if err != nil {
		return err
	}

	switch args[0] {
	case "maintainer":
		fmt.Println("Welcome! This wizard will help you setup gittuf on your repository as a maintainer. Press Ctrl+C to quit at any time.")
		fmt.Println("To start, the wizard will attempt to autodetect your signing key from your Git configuration.")
		fmt.Println("To continue, press Enter.")
		if _, err = fmt.Scanln(); err != nil {
			return err
		}

		fmt.Println("Attempting to determine signing key from Git configuration...")
		signer, err := gittuf.LoadSignerFromGitConfig(repo)
		if err != nil {
			if !errors.Is(err, gittuf.ErrSigningKeyNotSpecified) {
				return err
			}

			fmt.Println("gittuf cannot autodetect your signing key.")
			fmt.Println("Please ensure that it is specified in your Git configuration.")
			fmt.Println("For more information, see https://gittuf.dev/")

			return err
		}

		fmt.Println("Initializing the root of trust...")
		// Detect whether the repository has a root of trust already set up
		err = repo.InitializeRoot(cmd.Context(), signer, true)
		if err != nil {
			// If it's an error other than a root of trust already existing,
			// return it
			if !errors.Is(err, gittuf.ErrCannotReinitialize) {
				return err
			}

			// Otherwise, inform the user that we will set up the repository
			// with the currently-existing root of trust
			fmt.Println("\nIt looks like another maintainer has already initialized gittuf for this repository.")
			fmt.Println("If this is correct, press Enter to continue.")
			fmt.Println("If you are NOT expecting this, press Ctrl+C NOW, and visit https://gittuf.dev/documentation/maintainers/root.")
			if _, err = fmt.Scanln(); err != nil {
				return err
			}

			// Configure the transport
			if err := configureTransport(repo); err != nil {
				return err
			}

			// Conclude
			fmt.Println("gittuf setup complete! Feel free to see the gittuf documentation at https://gittuf.dev/documentation/maintainers for more information.")
			fmt.Println("To add yourself to the root of trust, please see https://gittuf.dev/documentation/maintainers/root.")
			return nil
		}
		fmt.Println("\nIt looks you are the first maintainer to run 'gittuf setup'.")
		fmt.Println("If you'd like, gittuf can create a basic policy. You will be allowed to make changes to the default branch of the repository.")
		fmt.Println("However, you must still add other authorized users manually.")

		initializePolicy, err := promptUserConsent("Would you like to create a basic policy? [Y/n]")
		if err != nil {
			return err
		}

		if initializePolicy {
			// Load the user's public key in
			publicKey, err := gittuf.LoadPublicKeyFromGitConfig(repo)
			if err != nil {
				return err
			}

			fmt.Println("Initializing policy...")
			if err = repo.AddTopLevelTargetsKey(cmd.Context(), signer, publicKey, true); err != nil {
				return err
			}
			if err = repo.InitializeTargets(cmd.Context(), signer, policy.TargetsRoleName, true); err != nil {
				return err
			}
			if err = repo.AddPrincipalToTargets(cmd.Context(), signer, policy.TargetsRoleName, []tuf.Principal{publicKey}, true); err != nil {
				return err
			}

			// Determine the default branch of the repository
			defaultBranch, err := determineDefaultBranch(gitRepo)
			if err != nil {
				return nil
			}

			branchRefName, err := gitinterfaceRepo.AbsoluteReference(defaultBranch)
			if err != nil {
				return err
			}

			// Add the rule to protect the main branch
			fmt.Printf("Adding rule to protect the '%s' branch...\n", defaultBranch)
			principalNames := []string{publicKey.ID()}
			if err = repo.AddDelegation(cmd.Context(), signer, policy.TargetsRoleName, fmt.Sprintf("protect-%s", defaultBranch), principalNames, []string{fmt.Sprintf("git:%s", branchRefName)}, 1, true); err != nil {
				return err
			}
		}

		fmt.Println("Staging and applying the policy...")
		// Stage the policy
		if err = repo.StagePolicy(cmd.Context(), "", true, true); err != nil {
			return err
		}

		// Apply the policy
		if err = repo.ApplyPolicy(cmd.Context(), "", true, true); err != nil {
			return err
		}

		fmt.Println("Policy applied successfuly.")

		// Configure the transport
		if err := configureTransport(repo); err != nil {
			return err
		}

		// Conclude
		fmt.Println("gittuf setup complete!")
		fmt.Println("Please note that if you want to authorize other users to contribute to the repository, you must add them to the gittuf policy.")
		fmt.Println("See the gittuf documentation at https://gittuf.dev/documentation/maintainers for more information.")
		return nil
	case "contributor":
		fmt.Println("Welcome! This wizard will help you setup gittuf on your repository as a contributor. Press Ctrl+C to quit at any time.")
		fmt.Println()
		fmt.Println("To continue, press Enter.")
		if _, err = fmt.Scanln(); err != nil {
			return err
		}

		// Configure the transport
		if err := configureTransport(repo); err != nil {
			return err
		}

		// Conclude
		fmt.Println("gittuf setup complete! Feel free to see the gittuf documentation at https://gittuf.dev/documentation/contributors for more information.")
		return nil

	default:
		return fmt.Errorf("unrecognized repository role %s", args[0])
	}
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "setup",
		Short:             "Launch the gittuf setup wizard to quickly get started with gittuf on your repository",
		Long:              "The 'setup' command serves as an alternative to the manual setup process for gittuf, intended for rapid deployment of gittuf on repositories with a basic security policy",
		Args:              cobra.ExactArgs(1),
		DisableAutoGenTag: true,
		RunE:              o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}

// configureTransport handles the configuration of the gittuf transport for
// users.
func configureTransport(repo *gittuf.Repository) error {
	fmt.Println("gittuf can automatically record your pushes and verify the repository for you when you run git pull and push.")
	fmt.Println("gittuf will change the 'origin' remote and create a backup in case of issues.")

	enable, err := promptUserConsent("Would you like to enable this? [Y/n]")
	if err != nil {
		return err
	}

	if enable {
		gitRepo := repo.GetGitRepository()
		originRemote, err := gitRepo.GetRemoteURL("origin")
		if err != nil {
			return err
		}
		if err = gitRepo.AddRemote("origin-backup", originRemote); err != nil {
			return err
		}
		if err = gitRepo.SetRemote("origin", fmt.Sprintf("gittuf::%s", originRemote)); err != nil {
			return err
		}
	}

	return nil
}

// promptUserConsent is a helper function to handle the [Y/n] loops.
func promptUserConsent(prompt string) (bool, error) {
	for {
		fmt.Println(prompt)
		input := ""
		_, err := fmt.Scanln(&input)
		if err != nil {
			return false, err
		}

		switch input {
		case "", "y", "Y", "yes", "Yes":
			return true, nil
		case "n", "N", "no", "No":
			return false, nil
		}
	}
}

// determineDefaultBranch determines the default branch of the repository. We
// try to find "main" or "master". If none exist, ask the user.
func determineDefaultBranch(gitRepo *gitinterface.Repository) (string, error) {
	if _, err := gitRepo.GetReference("main"); err == nil {
		fmt.Println("The default branch has been detected as 'main'.")
		return "main", nil
	} else if _, err := gitRepo.GetReference("master"); err == nil {
		fmt.Println("The default branch has been detected as 'master'.")
		return "master", nil
	}

	fmt.Println("Unable to determine default branch. Please enter the default branch of the repository:")
	input := ""
	_, err := fmt.Scanln(&input)
	if err != nil {
		return "", err
	}
	return input, nil
}

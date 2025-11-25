// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
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
		"GitHub token",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	gitinterfaceRepo, err := gitinterface.LoadRepository(".")
	if err != nil {
		return err
	}

	fmt.Println("Welcome! This wizard will help you setup gittuf on your repository. Press Ctrl+C to quit at any time.")
	fmt.Println()
	fmt.Println("First, for root key management, the public Sigstore instance will be used. Remember the identity you choose to login with, as you will need it to update gittuf metadata later.")
	fmt.Println("To continue, press Enter.")
	_, err = fmt.Scanln()
	if err != nil {
		return err
	}

	signer := sigstore.NewSigner()

	// Initialize the root of trust with the Sigstore Signer
	if err = repo.InitializeRoot(cmd.Context(), signer, true); err != nil {
		return err
	}

	// Add the gittuf GitHub App as a trusted app
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}

	response, err := http.Get("https://raw.githubusercontent.com/gittuf/github-app/refs/heads/main/docs/hosted-app-key.pub")
	if err != nil {
		return err
	}
	defer response.Body.Close()

	appKeyFile, err := os.CreateTemp(tmpDir, "")
	if err != nil {
		return err
	}
	defer appKeyFile.Close()

	_, err = io.Copy(appKeyFile, response.Body)
	if err != nil {
		return err
	}

	filePath := appKeyFile.Name()
	appKey, err := gittuf.LoadPublicKey(filePath)
	if err != nil {
		return err
	}
	if err = repo.AddGitHubApp(cmd.Context(), signer, "https://gittuf.dev/github-app", appKey, true); err != nil {
		return err
	}

	fmt.Println("Next, a rule will be created to protect the default branch of your repository.")

	// Determine the default branch of the repository. We try to find "main" or "master". If none exist, ask the user.
	var defaultBranch string

	if _, err := gitinterfaceRepo.GetReference("main"); err == nil {
		fmt.Println("The default branch has been detected as 'main'.")
		defaultBranch = "main"
	} else if _, err := gitinterfaceRepo.GetReference("master"); err == nil {
		fmt.Println("The default branch has been detected as 'master'.")
		defaultBranch = "master"
	} else {
		fmt.Println("Unable to determine default branch. Please enter the default branch of the repository:")
		_, err := fmt.Scanln(&defaultBranch)
		if err != nil {
			return err
		}
	}
	fmt.Printf("Enter the GitHub usernames of all users who you would like to authorize to make changes to the '%s' branch.\n", defaultBranch)
	fmt.Println("When done, press Ctrl+D to continue.")

	var gitHubUsernames []string
	scanner := bufio.NewScanner(os.Stdin)

	for {
		notEmpty := scanner.Scan()
		if scanner.Err() != nil {
			return err
		}
		if !notEmpty {
			break
		}
		gitHubUsernames = append(gitHubUsernames, scanner.Text())
	}

	fmt.Println("Initializing policy...")
	// Extract the Fulcio identity from the Sigstore signer
	sigstoreKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	sigstorePerson, err := gittuf.LoadPublicKey(fmt.Sprintf("fulcio:%s", sigstoreKeyID))
	if err != nil {
		fmt.Println(sigstoreKeyID)
		return err
	}

	if err = repo.AddTopLevelTargetsKey(cmd.Context(), signer, sigstorePerson, true); err != nil {
		return err
	}
	if err = repo.InitializeTargets(cmd.Context(), signer, policy.TargetsRoleName, true); err != nil {
		return err
	}

	fmt.Printf("Adding rule to protect the '%s' branch...\n", defaultBranch)

	client, err := gittuf.GetGitHubClient("https://github.com", o.githubToken)
	if err != nil {
		return err
	}

	for _, username := range gitHubUsernames {
		fmt.Printf("Adding %s", username)
		user, _, err := client.Users.Get(cmd.Context(), username)
		if err != nil {
			return err
		}

		associatedIdentities := make(map[string]string)
		associatedIdentities["https://gittuf.dev/github-app"] = fmt.Sprintf("%s+%d", username, user.GetID())

		person := &tufv02.Person{
			PersonID:             username,
			AssociatedIdentities: associatedIdentities,
		}

		if err = repo.AddPrincipalToTargets(cmd.Context(), signer, policy.TargetsRoleName, []tuf.Principal{person}, true); err != nil {
			return err
		}
	}

	branchRefName, err := gitinterfaceRepo.AbsoluteReference(defaultBranch)
	if err != nil {
		return err
	}

	// Add the rule to protect the main branch
	if err = repo.AddDelegation(cmd.Context(), signer, policy.TargetsRoleName, fmt.Sprintf("protect-%s", defaultBranch), gitHubUsernames, []string{fmt.Sprintf("git:%s", branchRefName)}, 1, true); err != nil {
		return err
	}

	// Stage the policy
	if err = repo.StagePolicy(cmd.Context(), "", true, true); err != nil {
		return err
	}

	// Apply the policy
	if err = repo.ApplyPolicy(cmd.Context(), "", true, true); err != nil {
		return err
	}

	fmt.Println("gittuf setup complete! Feel free to see the gittuf documentation at gittuf.dev for more information.")
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Launch the gittuf setup wizard to quickly get started with gittuf on your repository",
		Long:  "The 'setup' command serves as an alternative to the manual setup process for gittuf, intended for rapid deployment of gittuf on repositories with a basic security policy",
		Args:  cobra.NoArgs,
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}

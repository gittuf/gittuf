// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/policy"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
)

const (
	addToRoot    int = iota // Add maintainer to root of trust
	addToTargets            // Add maintainer as trusted for toplevel targets file
	addToRule               // Add maintainer to rule protecting the default branch
)

// metadataDoneMsg is sent when the final command completes.
type metadataDoneMsg struct {
	steps []string
	err   error
}

// checkRootExists returns true if gittuf has already been initialized.
func checkRootExists(repo *gittuf.Repository) bool {
	_, err := repo.GetGitRepository().GetReference(policy.PolicyRef)
	return err == nil
}

// setupMaintainerChoices configures the gittuf policy on the repository as
// desired by the maintainer
func setupMaintainerChoices(ctx context.Context, repo *gittuf.Repository, signer sslibdsse.SignerVerifier, choices map[int]bool, alreadySetUp bool) tea.Cmd {
	return func() tea.Msg {
		// Load the maintainer's public key
		principal, err := gittuf.LoadPublicKeyFromGitConfig(repo)
		if err != nil {
			return metadataDoneMsg{err: err}
		}

		var steps []string

		// Handle the metadata choices in order
		if choices[addToRoot] {
			// Initialize root first and then add maintainer to root of trust.
			if !alreadySetUp {
				steps = append(steps, "gittuf trust init")
				err := repo.InitializeRoot(ctx, signer, true)
				if err != nil {
					return metadataDoneMsg{err: err}
				}
			}

			err := repo.AddRootKey(ctx, signer, principal, true)
			if err != nil {
				return metadataDoneMsg{err: err}
			}
		}

		if choices[addToTargets] {
			// Add maintainer as trusted for toplevel targets file
			if err := repo.AddTopLevelTargetsKey(ctx, signer, principal, true); err != nil {
				return metadataDoneMsg{err: err}
			}
		}

		if choices[addToRule] {
			// Add maintainer to rule protecting the default branch
			if !alreadySetUp {
				if err := repo.InitializeTargets(ctx, signer, policy.TargetsRoleName, true); err != nil {
					return metadataDoneMsg{err: err}
				}
				if err := repo.AddPrincipalToTargets(ctx, signer, policy.TargetsRoleName, []tuf.Principal{principal}, true); err != nil {
					return metadataDoneMsg{err: err}
				}
				if err := repo.AddDelegation(ctx, signer, policy.TargetsRoleName, "protect-main", []string{principal.ID()}, []string{"refs/heads/main"}, 1, true); err != nil {
					return metadataDoneMsg{err: err}
				}
			} else {
				// TODO: Add internal upgrades to return rule information
				if err := repo.UpdateDelegation(ctx, signer, policy.TargetsRoleName, "protect-main", []string{principal.ID()}, []string{"refs/heads/main"}, 1, true); err != nil {
					return metadataDoneMsg{err: err}
				}
			}
		}

		if err = repo.StagePolicy(ctx, "", true, true); err != nil {
			return metadataDoneMsg{err: err}
		}

		if !alreadySetUp {
			// Apply changes
			if err = repo.ApplyPolicy(ctx, "", true, true); err != nil {
				return metadataDoneMsg{err: err}
			}
		}

		return metadataDoneMsg{steps: steps, err: nil}
	}
}
